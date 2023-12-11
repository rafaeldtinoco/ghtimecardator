# GitHubTimeCardator

## Overview

This Go application provides a comprehensive summary of GitHub activities,
including issues and pull requests, for a specific user within a specified time
frame. It utilizes the GitHub API to fetch data and OpenAI's GPT-4 for
generating summaries. The tool is designed to offer different types of
summaries: executive, technical, and detailed, catering to various reporting
needs.

## Features

- Fetches GitHub activities like issues and pull requests.
- Identifies whether the user is the author of the issue/PR or just commenting.
- Summarizes activities using OpenAI's GPT-4, offering different summary types.
- Handles different GitHub events including comments and pull requests.
- Supports various time frames for reporting:
  - today
  - yesterday
  - last 3 days
  - this week
  - last week
  - this month
  - last month

## Installation

1. Ensure you have Go installed on your system.
2. Clone the repository: `git clone [repository URL]`.
3. Navigate to the project directory: `cd [project directory]`.
4. Install dependencies: `go get -d ./...`.

## Usage

1. Set environment variables:
   - `GITHUB_USER`: Your GitHub username.
   - `GITHUB_TOKEN`: Your GitHub token for API access.
   - `OPENAI_TOKEN`: Your OpenAI API token.
2. Run the application: `go run . [date] [summary type] [owner/repo]`.
   - `date`: Choose from `today`, `yesterday`, `last-3days`, `this-week`, `last-week`, `this-month`, `last-month`.
   - `summary type`: Choose from `executive`, `technical`, `detailed`.
   - `owner/repo`: Specify the GitHub repository in the format `owner/repository`.

## Examples

- Generate an executive summary for today's activities in the `username/repository` repo:

  ```console
  $ go install github.com/rafaeldtinoco/ghtimecardator@latest
  ```

  ```markdown
  **Features and Enhancements:**

  - Participated in the discussion and closure of issue #3576, which aimed to
    add file hashes to more events in the aquasecurity/tracee repository. The
    enhancement was implemented through pull requests #3715 and #3721.
  - Contributed to the discussion of issue #3711, suggesting the use of a trie
    data structure for the implementation of a feature to trace events and
    signatures exclusively from a specific pod namespace in Kubernetes.
  - Proposed an enhancement to the capture artifacts feature in issue #3714,
    suggesting a similar option to pcap capturing for read/write/exec captures.

  **Fixes:**

  - Authored pull request #3718, which fixed the event context timestamps
    normalization order in the aquasecurity/tracee project. The fix involved
    changing the sequence for `t.RegisterEventProcessor(events.All,
    t.normalizeEventCtxTimes)`.
  - Merged pull request #3713, which updated libbpfgo to v0.6.0-libbpf-1.3 and
    3rdparty/libbpf to v1.3.0, bringing new features and fixes as detailed in the
    libbpf v1.3.0 release notes.

  **Documentation:**

  - Authored and merged pull request #3721, which aimed to make pull request
    #3715 pass the document verification.

  **Reviews and Management:**

  - Reviewed and approved pull request #3713, which updated libbpfgo to
    v0.6.0-libbpf-1.3 and 3rdparty/libbpf to v1.3.0.
  - Participated in the discussion and closure of pull request #3715, providing
    instructions for the author to follow for future pull requests and addressing
    an issue with the pr.yaml comment and the Docker image used in the pull
    request.
  - Commented on pull request #3305, stating that a review would be conducted
    shortly. The pull request was about introducing support for policy versioning
    in both the userland and ebpf, facilitating runtime policy updates.

  **Tests:**

  - No specific test-related activities were reported in this period.
  ```

## Contributing

Contributions to improve this tool are welcome.

## License

Apache License, Version 2.0
