///bin/sh -c true; exec /usr/bin/env go run "$0" "$@"
//  vim:ts=4:sts=4:sw=4:noet
//
//  Author: Hari Sekhon
//  Date: 2024-09-08 02:24:45 +0200 (Sun, 08 Sep 2024)
//
//  https///github.com/HariSekhon/GitHub-Commit-Times-Graph
//
//  License: see accompanying Hari Sekhon LICENSE file
//
//  If you're using my code you're welcome to connect with me on LinkedIn and optionally send me feedback to help steer this or other code I publish
//
//  https://www.linkedin.com/in/HariSekhon
//

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v41/github"
	"golang.org/x/oauth2"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

// Fetch commits from a repository
func fetchCommits(client *github.Client, owner, repo string) ([]*github.RepositoryCommit, error) {
	commits, _, err := client.Repositories.ListCommits(context.Background(), owner, repo, nil)
	if err != nil {
		return nil, err
	}
	return commits, nil
}

// Fetch all public, non-fork repositories for a user
func fetchUserRepos(client *github.Client, user string) ([]*github.Repository, error) {
	opts := &github.RepositoryListOptions{Type: "public"}
	repos, _, err := client.Repositories.List(context.Background(), user, opts)
	if err != nil {
		return nil, err
	}

	// Filter out forked repositories
	var publicRepos []*github.Repository
	for _, repo := range repos {
		if !repo.GetFork() {
			publicRepos = append(publicRepos, repo)
		}
	}
	return publicRepos, nil
}

// Parse commit timestamps and aggregate commits by hour
func processCommits(commits []*github.RepositoryCommit, usernameFilter string) [24]int {
	hourlyCommits := [24]int{}

	for _, commit := range commits {
		if commit.Commit == nil || commit.Commit.Committer == nil {
			continue
		}

		// If filtering by user, skip commits not by the user
		if usernameFilter != "" && commit.Commit.Committer.GetName() != usernameFilter && commit.Commit.Committer.GetEmail() != usernameFilter {
			continue
		}

		commitTime := commit.Commit.Committer.GetDate()
		hour := commitTime.Hour()

		hourlyCommits[hour]++
	}

	return hourlyCommits
}

// Generate a bar graph and save it as a PNG file
func generateGraph(hourlyCommits [24]int, outputFile string) error {
	p := plot.New()

	p.Title.Text = "GitHub Commits by Hour"
	p.X.Label.Text = "Hour of Day"
	p.Y.Label.Text = "Number of Commits"

	// Create the bar data
	values := make(plotter.Values, 24)
	for i := 0; i < 24; i++ {
		values[i] = float64(hourlyCommits[i])
	}

	barChart, err := plotter.NewBarChart(values, vg.Points(20))
	if err != nil {
		return err
	}

	barChart.Color = plotter.DefaultLineStyle.Color
	p.Add(barChart)

	// Set the x-axis labels to represent hours (0-23)
	p.NominalX("00", "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23")

	// Save the plot to a PNG file
	if err := p.Save(10*vg.Inch, 4*vg.Inch, outputFile); err != nil {
		return err
	}
	fmt.Printf("Graph saved to %s\n", outputFile)
	return nil
}

// Show usage help
func showUsage() {
	fmt.Println("Usage: go run main.go [options] <repo1> <repo2> ...")
	fmt.Println("Options:")
	fmt.Println("  --user <username/email>    Filter commits by a specific username or email")
	fmt.Println("  -o, --output <file>        Output file for the graph (default: graph.png)")
	fmt.Println("  -h, --help                 Show this help message")
	fmt.Println("Repos:")
	fmt.Println("  Provide repositories in the format 'owner/repo'.")
	fmt.Println("  If only 'owner' is provided, fetches all public non-fork repos for that user.")
	os.Exit(0)
}

func main() {
	// Parse command-line flags
	var userFilter string
	var outputFile string
	flag.StringVar(&userFilter, "user", "", "Filter commits by a specific username or email")
	flag.StringVar(&outputFile, "output", "graph.png", "Output file for the graph")
	helpFlag := flag.Bool("help", false, "Show help")
	flag.BoolVar(helpFlag, "h", false, "Show help")
	flag.Parse()

	// Show help if -h or --help is provided
	if *helpFlag {
		showUsage()
	}

	// Get remaining arguments as repo inputs
	repoArgs := flag.Args()
	if len(repoArgs) == 0 {
		fmt.Println("Error: No repositories provided.")
		showUsage()
	}

	// GitHub token setup
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Fatal("Error: GITHUB_TOKEN environment variable is not set.")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// To store aggregated commit data per hour
	hourlyCommits := [24]int{}

	// Process each repository or user input
	for _, repoArg := range repoArgs {
		if strings.Contains(repoArg, "/") {
			// Assume the format is "owner/repo"
			parts := strings.Split(repoArg, "/")
			if len(parts) != 2 {
				log.Fatalf("Invalid repository format: %s (expected 'owner/repo')", repoArg)
			}
			owner := parts[0]
			repo := parts[1]

			fmt.Printf("Fetching commits from %s/%s...\n", owner, repo)
			commits, err := fetchCommits(client, owner, repo)
			if err != nil {
				log.Fatalf("Error fetching commits from %s/%s: %v", owner, repo, err)
			}

			repoHourlyCommits := processCommits(commits, userFilter)
			for hour := 0; hour < 24; hour++ {
				hourlyCommits[hour] += repoHourlyCommits[hour]
			}
		} else {
			// Assume it's a GitHub username, fetch all public non-fork repos
			user := repoArg
			fmt.Printf("Fetching public non-fork repos for user %s...\n", user)
			repos, err := fetchUserRepos(client, user)
			if err != nil {
				log.Fatalf("Error fetching repos for user %s: %v", user, err)
			}

			for _, repo := range repos {
				fmt.Printf("Fetching commits from %s/%s...\n", user, repo.GetName())
				commits, err := fetchCommits(client, user, repo.GetName())
				if err != nil {
					log.Fatalf("Error fetching commits from %s/%s: %v", user, repo.GetName(), err)
				}

				repoHourlyCommits := processCommits(commits, userFilter)
				for hour := 0; hour < 24; hour++ {
					hourlyCommits[hour] += repoHourlyCommits[hour]
				}
			}
		}
	}

	// Generate the graph and save it as a PNG file
	if err := generateGraph(hourlyCommits, outputFile); err != nil {
		log.Fatalf("Error generating graph: %v", err)
	}
}
