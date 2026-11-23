package jenkins

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: "jenkins_list_jobs", Access: mcp.AccessRead,
		Description: "List one level of Jenkins jobs, folders and CI/CD pipeline status. Start here to discover a job or descend a folder without installing a new Jenkins plugin. Returns name, _class, color and lastBuild number/result/timestamp with a bounded offset/limit window; possibly_more means a full window, not a guaranteed next page or total. No logs, parameters or writes.",
		Parameters: map[string]string{
			"folder_path": "Optional ordered JSON array of exact item-name segments; [] or omitted lists root. Example [\"team\",\"pipelines\"]. A multibranch name feature/foo is ONE segment, not a URL or split path.",
			"offset":      "Selection window offset, integer >= 0 (default 0); concurrent inventory changes can shift windows",
			"limit":       "Selection window size, integer 1-50 (default 20); oversized provider responses fail, never silently truncate",
		},
	},
	{
		Name: "jenkins_get_job", Access: mcp.AccessRead,
		Description: "Get an exact Jenkins job or CI/CD pipeline summary and a bounded recent builds window. Use after jenkins_list_jobs for buildable, inQueue, lastBuild and builds number/result/timestamp/duration/building. possibly_more is only a full-window hint; no total or snapshot guarantee. No logs, parameters or writes.",
		Parameters: map[string]string{
			"job_path": "Required nonempty ordered JSON array of exact item-name segments from discovery, e.g. [\"team\",\"feature/foo\"]. Multibranch feature/foo remains ONE segment; never pass a URL.",
			"offset":   "Build selection window offset, integer >= 0 (default 0)",
			"limit":    "Build selection window size, integer 1-50 (default 20)",
		},
		Required: []string{"job_path"},
	},
	{
		Name: "jenkins_get_build", Access: mcp.AccessRead,
		Description: "Get one exact Jenkins CI/CD pipeline build result, phase (building/completed), timestamp, duration and estimatedDuration. Use after jenkins_get_job with a discovered positive build number. Null result is valid during an in-progress build. No console logs, actions, changeSets, parameters or writes.",
		Parameters: map[string]string{
			"job_path":     "Required nonempty ordered JSON array of exact item-name segments, e.g. [\"team\",\"feature/foo\"]. Encode nothing yourself; a multibranch name is ONE segment.",
			"build_number": "Required positive integer build number from lastBuild or builds; not a symbolic alias or URL",
		},
		Required: []string{"job_path", "build_number"},
	},
}
