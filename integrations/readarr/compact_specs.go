package readarr

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	mcp.ToolName("readarr_list_books"): {
		"id", "title", "authorTitle", "releaseDate", "monitored",
		"grabbed", "statistics.bookFileCount", "statistics.percentOfBooks",
		"ratings.value",
	},
	mcp.ToolName("readarr_get_book"): {
		"id", "title", "authorTitle", "releaseDate", "monitored",
		"overview", "grabbed", "pageCount",
		"ratings.value", "ratings.votes",
		"statistics.bookFileCount", "statistics.sizeOnDisk",
		"editions[].id", "editions[].title", "editions[].monitored", "editions[].grabbed",
		"author.id", "author.authorName",
	},
	mcp.ToolName("readarr_search_books"): {
		"foreignId", "author.authorName",
		"book.id", "book.title", "book.releaseDate",
		"book.ratings.value",
	},
	mcp.ToolName("readarr_list_authors"): {
		"id", "authorName", "monitored", "status", "path",
		"qualityProfileId", "metadataProfileId",
		"statistics.bookFileCount", "statistics.bookCount", "statistics.percentOfBooks",
		"ratings.value",
	},
	mcp.ToolName("readarr_get_author"): {
		"id", "authorName", "monitored", "status", "path", "overview",
		"qualityProfileId", "metadataProfileId",
		"statistics.bookFileCount", "statistics.bookCount", "statistics.sizeOnDisk",
		"ratings.value", "ratings.votes",
	},
	mcp.ToolName("readarr_get_calendar"): {
		"id", "title", "authorTitle", "releaseDate", "monitored",
		"grabbed",
	},
	mcp.ToolName("readarr_get_missing"): {
		"records[].id", "records[].title", "records[].authorTitle",
		"records[].releaseDate", "records[].monitored",
		"page", "pageSize", "totalRecords",
	},
	mcp.ToolName("readarr_get_cutoff"): {
		"records[].id", "records[].title", "records[].authorTitle",
		"records[].releaseDate", "records[].monitored",
		"page", "pageSize", "totalRecords",
	},
	mcp.ToolName("readarr_get_queue"): {
		"records[].id", "records[].title", "records[].status",
		"records[].timeleft", "records[].estimatedCompletionTime",
		"records[].protocol", "records[].downloadClient",
		"records[].bookId", "records[].authorId",
		"page", "pageSize", "totalRecords",
	},
	mcp.ToolName("readarr_get_history"): {
		"records[].id", "records[].bookId", "records[].authorId",
		"records[].eventType", "records[].date",
		"records[].sourceTitle",
		"page", "pageSize", "totalRecords",
	},
	mcp.ToolName("readarr_get_history_author"): {
		"id", "bookId", "authorId", "eventType", "date", "sourceTitle",
	},
	mcp.ToolName("readarr_get_history_since"): {
		"id", "bookId", "authorId", "eventType", "date", "sourceTitle",
	},
	mcp.ToolName("readarr_list_commands"): {
		"id", "name", "commandName", "status", "started", "ended",
		"stateChangeTime",
	},
	mcp.ToolName("readarr_get_command"): {
		"id", "name", "commandName", "status", "started", "ended",
		"stateChangeTime", "body",
	},
	mcp.ToolName("readarr_list_root_folders"): {
		"id", "path", "name", "accessible", "freeSpace",
	},
	mcp.ToolName("readarr_list_quality_profiles"): {
		"id", "name", "cutoff.id", "cutoff.name",
	},
	mcp.ToolName("readarr_list_metadata_profiles"): {
		"id", "name", "minPopularity",
	},
	mcp.ToolName("readarr_list_tags"): {
		"id", "label",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("readarr: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
