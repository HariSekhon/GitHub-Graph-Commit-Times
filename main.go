///bin/sh -c true; exec /usr/bin/env go run "$0" "$@"
//  vim:ts=4:sts=4:sw=4:noet
//  args: harisekhon
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

//go:build !debug
// +build !debug

package main

import (
	"context"
	"flag"
	"fmt"
	"image/color"
	"log"
	"os"
	"strings"

	"github.com/google/go-github/v41/github"
	"golang.org/x/oauth2"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/vgsvg"
	"gonum.org/v1/plot/vg/draw"
)

// Fetch all commits from a repository with pagination
func fetchCommits(client *github.Client, owner, repo string) ([]*github.RepositoryCommit, error) {
	var allCommits []*github.RepositoryCommit
	opt := &github.CommitsListOptions{ListOptions: github.ListOptions{PerPage: 100}}

	for {
		commits, resp, err := client.Repositories.ListCommits(context.Background(), owner, repo, opt)
		if err != nil {
			return nil, err
		}

		allCommits = append(allCommits, commits...)

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allCommits, nil
}

// Fetch all public, non-fork repositories for a user with pagination
func fetchUserRepos(client *github.Client, user string) ([]*github.Repository, error) {
	var allRepos []*github.Repository
	opt := &github.RepositoryListOptions{Type: "public", ListOptions: github.ListOptions{PerPage: 100}}

	for {
		repos, resp, err := client.Repositories.List(context.Background(), user, opt)
		if err != nil {
			return nil, err
		}

		for _, repo := range repos {
			if !repo.GetFork() {
				allRepos = append(allRepos, repo)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allRepos, nil
}

// Process commits and aggregate by hour
func processCommits(commits []*github.RepositoryCommit, usernameFilter string) [24]int {
	hourlyCommits := [24]int{}

	for _, commit := range commits {
		if commit.Commit == nil || commit.Commit.Committer == nil {
			continue
		}

		if usernameFilter != "" && commit.Commit.Committer.GetName() != usernameFilter && commit.Commit.Committer.GetEmail() != usernameFilter {
			continue
		}

		commitTime := commit.Commit.Committer.GetDate()
		hour := commitTime.Hour()

		hourlyCommits[hour]++
	}

	return hourlyCommits
}

// Generate a bar graph and save it as an SVG file
func generateGraph(hourlyCommits [24]int, outputFile, usernameFilter, repoFilter string) error {
	p := plot.New()

	// Set the graph title based on the filter type
	if usernameFilter != "" {
		p.Title.Text = fmt.Sprintf("GitHub Commits by Hour for %s", usernameFilter)
	} else if repoFilter != "" {
		p.Title.Text = fmt.Sprintf("GitHub Commits by Hour for Repos under %s", repoFilter)
	} else {
		p.Title.Text = "GitHub Commits by Hour"
	}
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

	// Set the bar color to blue
	barChart.Color = color.RGBA{R: 0, G: 0, B: 255, A: 255} // Blue color

	p.Add(barChart)

	// Set the x-axis labels to represent hours (0-23)
	p.NominalX("00", "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23")

	// Create the SVG canvas with the correct width and height
	width := vg.Inch * 10
	height := vg.Inch * 4
	canvas := vgsvg.New(width, height)

	// Draw the plot on the canvas
	p.Draw(draw.New(canvas))

	// Save the SVG to the file
	w, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = canvas.WriteTo(w)
	if err != nil {
		return err
	}

	fmt.Printf("Graph saved to %s\n", outputFile)
	return nil
}

// Show usage help
func showUsage() {
	fmt.Println("Usage: go run main.go [options] [<username>] <repo1> <repo2> ...")
	fmt.Println("Options:")
	fmt.Println("  --user <username/email>    Filter commits by a specific username or email")
	fmt.Println("  --repo <owner/repo>        Filter commits by specific repository (format: 'owner/repo')")
	fmt.Println("  -o, --output <file>        Output file for the graph (default: graph.svg)")
	fmt.Println("  -h, --help                 Show this help message")
	fmt.Println("Repos:")
	fmt.Println("  Provide repositories in the format 'owner/repo'.")
	fmt.Println("  If only 'owner' is provided, fetches all public non-fork repos for that user.")
	os.Exit(0)
}

func main() {
	var userFilter, repoFilter, outputFile string
	flag.StringVar(&userFilter, "user", "", "Filter commits by a specific username or email")
	flag.StringVar(&repoFilter, "repo", "", "Filter commits by specific repository (format: 'owner/repo')")
	flag.StringVar(&outputFile, "output", "graph.svg", "Output file for the graph")
	helpFlag := flag.Bool("help", false, "Show help")
	flag.BoolVar(helpFlag, "h", false, "Show help")
	flag.Parse()

	if *helpFlag {
		showUsage()
	}

	repoArgs := flag.Args()
	if len(repoArgs) == 0 && repoFilter == "" && userFilter == "" {
		fmt.Println("Error: No repositories, username, or repo filter provided.")
		showUsage()
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Fatal("Error: GITHUB_TOKEN environment variable is not set.")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	hourlyCommits := [24]int{}

	if repoFilter != "" {
		parts := strings.Split(repoFilter, "/")
		if len(parts) != 2 {
			log.Fatalf("Invalid repository format: %s (expected 'owner/repo')", repoFilter)
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
		for _, repoArg := range repoArgs {
			if strings.Contains(repoArg, "/") {
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
	}

	if err := generateGraph(hourlyCommits, outputFile, userFilter, repoFilter); err != nil {
		log.Fatalf("Error generating graph: %v", err)
	}
}
