package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/google/go-github/v41/github"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/schema"

	"golang.org/x/oauth2"
)

// Events is used to unmarshal the list of events from the GitHub API.

func getEnvOrExit(key string) string {
	value := os.Getenv(key)
	if value == "" {
		fmt.Printf("%s environment variable not set.\n", key)
		os.Exit(1)
	}
	return value
}

const (
	ObjectIssue        = "issue"
	ObjectIssueComment = "issue comment"
	ObjectPR           = "pull request"
	ObjectPRComment    = "pull request comment"
)

type id int

type metadata struct {
	eventId     id     // issue or pull request number
	url         string // issue or pull request URL
	title       string // issue or pull request title
	description string // issue or pull request description
	author      bool   // true if I'm the author
}

type action struct {
	action  string // create, edit, delete, etc.
	object  string // issue, pull request, issue comment, pull request comment, etc.
	content string // the content of the action (summarized)
}

type work struct {
	issues  map[id]*metadata
	pulls   map[id]*metadata
	actions map[id][]*action
	user    string
}

func (w *work) addIssue(issue *github.Issue) {
	place := w.issues

	if issue.IsPullRequest() { // sometimes issues are pull requests
		place = w.pulls
	}

	id := id(issue.GetNumber())

	if _, ok := place[id]; ok {
		return
	}

	metadata := &metadata{
		eventId:     id,
		url:         issue.GetHTMLURL(),
		title:       issue.GetTitle(),
		description: descriptionSummary(issue.GetBody()),
		author:      issue.GetUser().GetLogin() == w.user,
	}

	place[id] = metadata
}

func (w *work) addPullRequest(pr *github.PullRequest) {
	id := id(pr.GetNumber())

	if _, ok := w.pulls[id]; ok {
		return
	}

	metadata := &metadata{
		eventId:     id,
		url:         pr.GetHTMLURL(),
		title:       pr.GetTitle(),
		description: descriptionSummary(pr.GetBody()),
		author:      pr.GetUser().GetLogin() == w.user,
	}

	w.pulls[id] = metadata
}

func (w *work) addAction(id id, a *action) {
	w.actions[id] = append(w.actions[id], a)
}

func (w *work) getAction(id id) []*action {
	if _, ok := w.actions[id]; !ok {
		return nil
	}
	return w.actions[id]
}

func (w *work) getIssue(id id) *metadata {
	if _, ok := w.issues[id]; ok {
		return w.issues[id]
	}
	return nil
}

func (w *work) getPR(id id) *metadata {
	if _, ok := w.pulls[id]; ok {
		return w.pulls[id]
	}
	return nil
}

func (w *work) getIssueOrPR(id id) *metadata {
	if _, ok := w.issues[id]; ok {
		return w.issues[id]
	}
	if _, ok := w.pulls[id]; ok {
		return w.pulls[id]
	}
	return nil
}

func (w *work) actionSummary(id id) string {
	role := actionSummaryString
	meta := w.getIssueOrPR(id)

	var instr string
	instr += fmt.Sprintf("Summary of #%d (%s) %s\n-\n", meta.eventId, meta.url, meta.title)
	instr += fmt.Sprintf("Author: %t\n-\n", meta.author)
	instr += fmt.Sprintf("Description: %s\n-\n", meta.description)
	instr += fmt.Sprintf("Actions: %d\n-\n", len(w.actions[id]))

	for _, action := range w.actions[id] {
		instr += fmt.Sprintf(
			"Action: %s\nObject: %s\nContent: %s\n-\n",
			action.action, action.object, action.content,
		)
	}

	return executeAI(role, instr)
}

// Main Program

var llm *openai.Chat

func main() {
	var err error

	githubUser := getEnvOrExit("GITHUB_USER")
	githubToken := getEnvOrExit("GITHUB_TOKEN")
	openAIToken := getEnvOrExit("OPENAI_TOKEN")

	flag.Usage = func() {
		fmt.Println("Usage: github [date] [summary type] [owner/repo]")
		fmt.Printf("  date: today, yesterday, last-3days, this-week, last-week, this-month, last-month\n")
		fmt.Printf("  type: executive, technical, detailed\n")
		fmt.Printf("  owner/repo: the repository to report on\n")
		flag.PrintDefaults()
	}

	flag.Parse()
	args := flag.Args()

	if len(args) < 2 {
		flag.Usage()
		os.Exit(1)
	}

	summaryType := args[1]
	if summaryType != "executive" && summaryType != "technical" && summaryType != "detailed" {
		fmt.Println("Invalid summary type:", summaryType)
		flag.Usage()
		os.Exit(1)
	}

	wantedRepo := args[2]
	if wantedRepo != "" {
		wantedRepo = strings.ToLower(wantedRepo)
		if !strings.Contains(wantedRepo, "/") {
			fmt.Println("Invalid owner/repo:", wantedRepo)
			flag.Usage()
			os.Exit(1)
		}
	}

	// Get the begin date
	beginDate, err := pickDate(args[0])
	if err != nil {
		fmt.Println(err)
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()

	// Create an OpenAI client
	llm, err = openai.NewChat(
		openai.WithModel("gpt-4"),
		openai.WithToken(openAIToken),
	)
	if err != nil {
		fmt.Println("Error creating OpenAI client:", err)
		os.Exit(1)
	}

	// Create a GitHub client
	tokenSrc := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
	tokenClient := oauth2.NewClient(ctx, tokenSrc)
	ghClient := github.NewClient(tokenClient)
	opt := &github.ListOptions{PerPage: 100}

	// Get the GitHub username
	user, _, err := ghClient.Users.Get(ctx, "")
	if err != nil {
		fmt.Println("Error fetching user:", err)
		return
	}

	// Initialize the work
	work := &work{
		issues:  make(map[id]*metadata),
		pulls:   make(map[id]*metadata),
		actions: make(map[id][]*action),
		user:    user.GetLogin(),
	}

	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)

	// Get all the events for the user
	myEvents := ghClient.Activity.ListEventsPerformedByUser
	for {
		s.Prefix = fmt.Sprintf("Fetching events... page %d ", opt.Page)
		s.Start()

		ghEvents, resp, err := myEvents(ctx, githubUser, false, opt)
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, event := range ghEvents {
			eventTime := event.GetCreatedAt()
			if eventTime.Before(beginDate) {
				break
			}
			repoName := event.GetRepo().GetName()
			if wantedRepo != "" && repoName != wantedRepo {
				continue
			}

			handleEvent(work, event)
		}

		if resp.NextPage == 0 {
			opt.Page = resp.FirstPage
			break
		}
		opt.Page = resp.NextPage

		s.Stop()
	}

	// Create a big report (will be used for the timecard)

	s.Prefix = "Creating report "
	s.Start()
	report := ""
	report += fmt.Sprintf("\nIssues:\n\n")
	for _, issue := range work.issues {
		id := issue.eventId
		result := work.actionSummary(id)
		report += fmt.Sprintf("Issue: #%d (%s) %s\n", issue.eventId, issue.url, issue.title)
		report += fmt.Sprintf("Description: %s\n", result)
	}
	s.Stop()

	s.Prefix = "Creating timecard "
	s.Start()
	report += fmt.Sprintf("\nPulls:\n\n")
	for _, pull := range work.pulls {
		id := pull.eventId
		result := work.actionSummary(id)
		report += fmt.Sprintf("PR: #%d (%s) %s\n", pull.eventId, pull.url, pull.title)
		report += fmt.Sprintf("Description: %s\n", result)
	}
	s.Stop()

	// Create the timecard
	fmt.Println(timecardSummary(summaryType, report))
}

// handleEvent is called for each event and adds it to the work.
func handleEvent(w *work, e *github.Event) {
	pay, err := e.ParsePayload()
	if err != nil {
		fmt.Println("Error parsing payload:", err)
	}

	switch v := pay.(type) {
	//
	// General Events
	//
	case *github.IssuesEvent:
		w.addIssue(v.GetIssue())
		w.addAction(id(v.GetIssue().GetNumber()),
			&action{
				action:  v.GetAction(),
				object:  ObjectIssue,
				content: descriptionSummary(v.GetIssue().GetBody()),
			})
	case *github.PullRequestEvent:
		realAction := v.GetAction()
		if realAction == "closed" {
			if v.GetPullRequest().GetMerged() {
				realAction = "merged"
			} else {
				realAction = "closed"
			}
		}
		w.addPullRequest(v.GetPullRequest())
		w.addAction(id(v.GetPullRequest().GetNumber()),
			&action{
				action:  realAction,
				object:  ObjectPR,
				content: descriptionSummary(v.GetPullRequest().GetBody()),
			})
	//
	// Related to Comments, Reviews, etc.
	//
	case *github.IssueCommentEvent:
		w.addIssue(v.GetIssue())
		w.addAction(id(v.GetIssue().GetNumber()),
			&action{
				action:  v.GetAction(),
				object:  ObjectIssueComment,
				content: descriptionSummary(v.GetComment().GetBody()),
			})
	case *github.PullRequestReviewEvent:
		w.addPullRequest(v.GetPullRequest())
		w.addAction(id(v.GetPullRequest().GetNumber()),
			&action{
				action:  v.GetAction(),
				object:  ObjectPRComment,
				content: descriptionSummary(v.GetReview().GetBody()),
			})
	case *github.PullRequestReviewCommentEvent:
		w.addPullRequest(v.GetPullRequest())
		w.addAction(id(v.GetPullRequest().GetNumber()),
			&action{
				action:  v.GetAction(),
				object:  ObjectPRComment,
				content: descriptionSummary(v.GetComment().GetBody()),
			})
	//
	// TODO
	//
	case *github.CommitCommentEvent:
	case *github.CreateEvent:
	case *github.DeleteEvent:
	case *github.MilestoneEvent:
	case *github.PackageEvent:
	case *github.PushEvent:
	case *github.ReleaseEvent:
	case *github.RepositoryEvent:
	case *github.RepositoryVulnerabilityAlertEvent:
	default:
		fmt.Println("Unknown event type:", e.GetType())
	}
}

// Summarization

// timecardSummary returns a summary of the timecard using openai.
func timecardSummary(summaryType, report string) string {
	role := timecardSummaryString

	switch summaryType {
	case "executive":
		role += timecardSummaryExecutive
	case "technical":
		role += timecardSummaryTechnical
	case "detailed":
		role += timecardSummaryExecutive + timecardSummaryTechnical
	}

	return executeAI(role, report)
}

// descriptionSummary returns a summary of the description using openai.
func descriptionSummary(text string) string {
	role := "You are a BOT that rewrites GitHub Issue and PR descriptions."
	instr := "Rewrite description below in couple of lines:\n\n" + text
	return executeAI(role, instr)
}

// executeAI is a helper function that calls the openai api.
func executeAI(role, instr string) string {
	answer, _ := llm.Call(
		context.Background(),
		[]schema.ChatMessage{
			schema.SystemChatMessage{Content: role},
			schema.HumanChatMessage{Content: instr},
		},
		llms.WithTemperature(0.2),
		llms.WithMaxLength(180),
	)
	if answer == nil {
		return ""
	}
	return answer.GetContent()
}

// Date Helpers

// pickDate returns the begin date for the report.
func pickDate(arg string) (time.Time, error) {
	var beginDate time.Time

	switch arg {
	case "today":
		beginDate = time.Now()
	case "yesterday":
		beginDate = time.Now().AddDate(0, 0, -1)
	case "last-3days":
		beginDate = time.Now().AddDate(0, 0, -3)
	case "this-week":
		// Assuming the week starts on Monday
		today := time.Now()
		offset := int(today.Weekday()) - int(time.Monday)
		if offset < 0 {
			offset += 7 // Handle Sunday
		}
		beginDate = today.AddDate(0, 0, -offset)
	case "last-week":
		today := time.Now()
		offset := int(today.Weekday()) + 6 // Go back to the previous Monday
		beginDate = today.AddDate(0, 0, -offset)
	case "this-month":
		today := time.Now()
		beginDate = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	case "last-month":
		today := time.Now()
		firstOfThisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
		beginDate = firstOfThisMonth.AddDate(0, -1, 0)
	default:
		return time.Now(), fmt.Errorf("invalid argument")
	}

	return beginDate, nil
}

//
// Prompt Strings
//

var actionSummaryString string = `
You will be given a summary of a GitHub Issue or PR and a series of actions made
by me on it. They will be in the form of:

Summary of the issue or PR (check URL string to see if it is an issue or PR)
-
Author: true or false (if I'm the author of the issue or PR)
-
Action: create, edit, delete, etc.
Object: issue, pull request, issue comment, pull request comment/review.
Content: description.
-
...

Your job is to describe what I did in this issue, or pull request, taking into
consideration the issue description AND the series of actions, objects and
description given in the form above.

Note: I'm creading issues and pull requests, but I'm also commenting in other
people's issues and pull requests (and sometimes replying above quoted text).
So, you should be able to differentiate whether I'm the author of the issue or
PR, or if I'm just commenting on it (or reviewing it).
`

var timecardSummaryString string = `
You will be given a complete report of all the issues and pull requests I
created or commented on in a certain period of time. The report will be in the
form of:

Issues:
Issue: number (URL) title
Description: summary of what I did in the issue
Issue:
...

Pulls:
PR: number (URL) title
Description: summary of what I did in the pull request
PR:
...
`

var timecardSummaryExecutive string = `
Provide an executive summary of the report below. Don't try to sell yourself,
just provide the facts. Differentiate between features, fixes or chores. The
executive summary should be no more than 3-4 sentences.
`
var timecardSummaryTechnical string = `
Provide a technical summary of the report below. Don't try to sell yourself,
just provide the facts. The technical summary should be written in a technical
language. Differentiate between features, fixes, docs, tests, management, ...
Split the technical summary into sections, if needed. Use emojis to
differentiate between sections.
`
