# (Experimental) slack-chromedp

This is an experimental tool for GPTScript. It uses the ChromeDP library to drive Google Chrome and do things using Slack.

Unfortunately, it does not work if Chrome runs in headless mode, so you will see your browser pop up when using this tool.

**Capabilities**: This tool can send messages in a Slack workspace, list all users, list all channels, and search for messages in a workspace.

## Usage

This tool does not require any API key. It simply requires that you are logged into Slack in your Chrome browser.

By default, this tool will use the Default user profile in Chrome. If you want to use a different profile, then the LLM
will need to set the `dataDir` argument to the proper directory where that profile's data are stored. You can ask it to
do this simply by telling it to set the argument to a particular directory.

You will also need to tell the LLM the URL of your Slack workspace. Here is an example:

```
tools: github.com/g-linville/slack-chromedp

Search for messages about Disney in the random channel in the Slack workspace at https://<my workspace>.slack.com
```

## License

Copyright (c) 2024 [Acorn Labs, Inc.](http://acorn.io/)

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the [License](LICENSE) for the specific language governing permissions and limitations under the License.
