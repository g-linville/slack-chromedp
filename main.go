package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

type args struct {
	WorkspaceURL string `json:"workspaceURL"`
	DataDir      string `json:"dataDir"`
	Query        string `json:"query"`
	Channel      string `json:"channel"`
	Message      string `json:"message"`
}

type message struct {
	channel string
	user    string
	time    string
	text    string
}

func (m message) print() {
	fmt.Printf("[%s] %s in #%s: %s\n", m.time, m.user, m.channel, m.text)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) != 3 {
		panic("Usage: " + os.Args[0] + " <command> <JSON parameters>")
	}

	var a args
	if err := json.Unmarshal([]byte(os.Args[2]), &a); err != nil {
		panic("Failed to unmarshal args: " + err.Error())
	}

	// Determine the Chrome data directory
	if a.DataDir == "" {
		user, err := user.Current()
		if err != nil {
			panic("Failed to get current user: " + err.Error())
		}

		switch runtime.GOOS {
		case "darwin":
			a.DataDir = user.HomeDir + "/Library/Application Support/Google/Chrome"
		case "linux":
			a.DataDir = user.HomeDir + "/.config/google-chrome"
		case "windows":
			a.DataDir = user.HomeDir + "/AppData/Local/Google/Chrome/User Data"
		default:
			panic("Unsupported OS: " + runtime.GOOS)
		}
	}

	switch os.Args[1] {
	case "search":
		if err := searchMessages(ctx, a); err != nil {
			panic("Failed to search messages: " + err.Error())
		}
	case "list_channels":
		if err := listChannels(ctx, a); err != nil {
			panic("Failed to list channels: " + err.Error())
		}
	case "list_users":
		if err := listUsers(ctx, a); err != nil {
			panic("Failed to list users: " + err.Error())
		}
	case "send_message":
		if err := sendMessage(ctx, a); err != nil {
			panic("Failed to send message: " + err.Error())
		}
	default:
		panic("Unknown command: " + os.Args[1])
	}
}

func searchMessages(parentCtx context.Context, a args) error {
	withTimeout, cancel := context.WithTimeout(parentCtx, 15*time.Second)
	defer cancel()
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.UserDataDir(a.DataDir), chromedp.Flag("headless", false))
	ctx, stop := chromedp.NewExecAllocator(withTimeout, opts...)
	defer stop()

	ctx, stop = chromedp.NewContext(ctx)
	defer stop()

	var nodes []*cdp.Node
	err := chromedp.Run(ctx,
		chromedp.Navigate(a.WorkspaceURL),
		chromedp.Sleep(2*time.Second),
		chromedp.Click("button.p-top_nav__search", chromedp.NodeVisible),
		chromedp.Click("button.p-top_nav__search", chromedp.NodeVisible), // Sometimes we have to click the button twice to get the search to pop up
		chromedp.WaitVisible("div.texty_single_line_input > div.ql-editor"),
		chromedp.Evaluate("document.querySelector('div.texty_single_line_input > div.ql-editor > p').innerText = '';", nil), // Run JavaScript to clear the search box
		chromedp.KeyEvent(a.Query+kb.Enter),                                                                                 // Type in the search query
		chromedp.Sleep(time.Second),
		chromedp.Nodes("div.c-message_group, div.c-search__blank_state_title", &nodes, chromedp.Populate(-1, false)))
	if err != nil {
		return err
	}

	// c-search__blank_state_title is the class of the div that appears when no messages are found
	if len(nodes) == 0 || (len(nodes) == 1 && strings.Contains(nodes[0].AttributeValue("class"), "c-search__blank_state_title")) {
		fmt.Println("no messages found")
		return nil
	}

	fmt.Println("Messages:")

	for _, node := range nodes {
		var result message
		channelNode := recursiveChildClassSelect(node, "c-channel_entity__name")
		if channelNode != nil && len(channelNode.Children) == 1 {
			result.channel = channelNode.Children[0].NodeValue
		}

		userNode := recursiveChildClassSelect(node, "c-message__sender_button")
		if userNode != nil && len(userNode.Children) == 1 {
			result.user = userNode.Children[0].NodeValue
		}

		timeNode := recursiveChildClassSelect(node, "c-timestamp__label")
		if timeNode != nil && len(timeNode.Children) == 1 {
			result.time = timeNode.Children[0].NodeValue
		}

		textNode := recursiveChildClassSelect(node, "p-rich_text_section") // Single-line messages will have this class
		if textNode != nil {
			result.text = recursiveNodeValue(textNode)
		} else {
			textNode = recursiveChildClassSelect(node, "c-search_message__body") // Multi-line messages will have this class
			if textNode != nil {
				result.text = recursiveNodeValue(textNode)
			}
		}

		result.print()
	}

	return nil
}

func recursiveChildClassSelect(node *cdp.Node, class string) *cdp.Node {
	classes := strings.Split(node.AttributeValue("class"), " ")
	if slices.Contains(classes, class) {
		return node
	}

	for _, c := range node.Children {
		if n := recursiveChildClassSelect(c, class); n != nil {
			return n
		}
	}

	return nil
}

func recursiveNodeValue(node *cdp.Node) string {
	var value string
	if node.NodeValue != "" {
		value = node.NodeValue
	}

	classes := strings.Split(node.AttributeValue("class"), " ")
	if slices.Contains(classes, "c-mrkdwn__br") { // This represents a newline in a multi-line message
		value += "\n"
	}

	for _, c := range node.Children {
		value += recursiveNodeValue(c)
	}

	return value
}

func listChannels(parentCtx context.Context, a args) error {
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.UserDataDir(a.DataDir), chromedp.Flag("headless", false))
	ctx, stop := chromedp.NewExecAllocator(parentCtx, opts...)
	defer stop()

	ctx, stop = chromedp.NewContext(ctx)
	defer stop()

	var nodes []*cdp.Node
	err := chromedp.Run(ctx,
		chromedp.Navigate(a.WorkspaceURL),
		chromedp.Click("#home", chromedp.ByID, chromedp.NodeVisible),
		chromedp.Nodes("div.p-channel_sidebar__channel[data-qa-channel-sidebar-channel-type=\"channel\"]", &nodes, chromedp.NodeVisible, chromedp.Populate(-1, false)))
	if err != nil {
		return err
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no channels found")
	}

	for _, node := range nodes {
		nameNode := recursiveChildClassSelect(node, "p-channel_sidebar__name")
		if nameNode != nil && len(nameNode.Children) == 1 {
			if len(nameNode.Children[0].Children) == 1 {
				fmt.Printf("#%s\n", nameNode.Children[0].Children[0].NodeValue)
			}
		}
	}

	return nil
}

func listUsers(parentCtx context.Context, a args) error {
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.UserDataDir(a.DataDir), chromedp.Flag("headless", false))
	ctx, stop := chromedp.NewExecAllocator(parentCtx, opts...)
	defer stop()

	ctx, stop = chromedp.NewContext(ctx)
	defer stop()

	var location string
	err := chromedp.Run(ctx,
		chromedp.Navigate(a.WorkspaceURL),
		chromedp.Click("#home", chromedp.ByID, chromedp.NodeVisible),
		chromedp.Sleep(2*time.Second),
		chromedp.Location(&location))
	if err != nil {
		return err
	}

	// Replace everything after the last slash with "people"
	location = location[:strings.LastIndex(location, "/")+1] + "people"

	var nodes []*cdp.Node
	err = chromedp.Run(ctx,
		chromedp.Navigate(location),
		chromedp.Nodes("span.p-browse_page_member_card_entity__name_text", &nodes, chromedp.NodeVisible, chromedp.Populate(-1, false)))

	for _, node := range nodes {
		if len(node.Children) == 1 {
			fmt.Printf("%s\n", node.Children[0].NodeValue)
		}
	}

	return nil
}

func sendMessage(parentCtx context.Context, a args) error {
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.UserDataDir(a.DataDir), chromedp.Flag("headless", false))
	ctx, stop := chromedp.NewExecAllocator(parentCtx, opts...)
	defer stop()

	ctx, stop = chromedp.NewContext(ctx)
	defer stop()

	mod := input.ModifierCtrl
	if runtime.GOOS == "darwin" {
		mod = input.ModifierCommand
	}

	if err := chromedp.Run(ctx,
		chromedp.Navigate(a.WorkspaceURL),
		chromedp.Sleep(2*time.Second),
		chromedp.KeyEvent("k", chromedp.KeyModifiers(mod)), // Use ctrl+k to bring up the channel/user search
		chromedp.WaitVisible("#something-off-text-node", chromedp.ByID),
		chromedp.KeyEvent(a.Channel+kb.Enter), // Type in the channel name
		chromedp.WaitNotPresent("#something-off-text-node", chromedp.ByID),
		chromedp.KeyEvent(a.Message+kb.Enter)); err != nil { // Type in the message
		return err
	}
	fmt.Println("message sent successfully")
	return nil
}
