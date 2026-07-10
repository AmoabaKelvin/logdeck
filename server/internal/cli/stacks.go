package cli

import (
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// composeProjectLabels mirrors the server's grouping: docker compose sets the
// com.docker.compose one, older podman-compose releases only the io.podman one.
var composeProjectLabels = []string{
	"com.docker.compose.project",
	"io.podman.compose.project",
}

// composeProject returns the compose project a container belongs to, or "".
func composeProject(labels map[string]string) string {
	for _, key := range composeProjectLabels {
		if value := labels[key]; value != "" {
			return value
		}
	}
	return ""
}

type stackSummary struct {
	Project    string   `json:"project"`
	Containers int      `json:"containers"`
	Running    int      `json:"running"`
	Hosts      []string `json:"hosts"`
}

// groupStacks groups containers by compose project label, client-side.
func groupStacks(containers []containerInfo) []stackSummary {
	byProject := map[string]*stackSummary{}
	hostsSeen := map[string]map[string]bool{}

	for _, c := range containers {
		project := composeProject(c.Labels)
		if project == "" {
			continue
		}
		summary, ok := byProject[project]
		if !ok {
			summary = &stackSummary{Project: project}
			byProject[project] = summary
			hostsSeen[project] = map[string]bool{}
		}
		summary.Containers++
		if c.State == "running" {
			summary.Running++
		}
		if c.Host != "" && !hostsSeen[project][c.Host] {
			hostsSeen[project][c.Host] = true
			summary.Hosts = append(summary.Hosts, c.Host)
		}
	}

	stacks := make([]stackSummary, 0, len(byProject))
	for _, summary := range byProject {
		sort.Strings(summary.Hosts)
		stacks = append(stacks, *summary)
	}
	sort.Slice(stacks, func(i, j int) bool { return stacks[i].Project < stacks[j].Project })
	return stacks
}

func newStacksCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "stacks",
		Short: "List compose projects with container counts and hosts",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			resp, err := a.fetchContainers(cmd.Context())
			if err != nil {
				return err
			}
			stacks := groupStacks(resp.Containers)

			if a.jsonOutput() {
				return a.printJSON(map[string]any{"stacks": stacks})
			}

			rows := make([][]string, 0, len(stacks))
			for _, s := range stacks {
				rows = append(rows, []string{
					s.Project,
					strconv.Itoa(s.Containers),
					strconv.Itoa(s.Running),
					strings.Join(s.Hosts, ","),
				})
			}
			renderTable(os.Stdout, []string{"PROJECT", "CONTAINERS", "RUNNING", "HOSTS"}, rows)
			return nil
		}),
	}
}
