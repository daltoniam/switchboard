package readarr

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	"readarr_list_books": {
		"id", "title", "authorTitle", "releaseDate", "monitored",
		"grabbed", "statistics.bookFileCount", "statistics.percentOfBooks",
		"ratings.value",
	},
	"readarr_get_book": {
		"id", "title", "authorTitle", "releaseDate", "monitored",
		"overview", "grabbed", "pageCount",
		"ratings.value", "ratings.votes",
		"statistics.bookFileCount", "statistics.sizeOnDisk",
		"editions[].id", "editions[].title", "editions[].monitored", "editions[].grabbed",
		"author.id", "author.authorName",
	},
	"readarr_search_books": {
		"foreignId", "author.authorName",
		"book.id", "book.title", "book.releaseDate",
		"book.ratings.value",
	},
	"readarr_list_authors": {
		"id", "authorName", "monitored", "status", "path",
		"qualityProfileId", "metadataProfileId",
		"statistics.bookFileCount", "statistics.bookCount", "statistics.percentOfBooks",
		"ratings.value",
	},
	"readarr_get_author": {
		"id", "authorName", "monitored", "status", "path", "overview",
		"qualityProfileId", "metadataProfileId",
		"statistics.bookFileCount", "statistics.bookCount", "statistics.sizeOnDisk",
		"ratings.value", "ratings.votes",
	},
	"readarr_get_calendar": {
		"id", "title", "authorTitle", "releaseDate", "monitored",
		"grabbed",
	},
	"readarr_get_missing": {
		"records[].id", "records[].title", "records[].authorTitle",
		"records[].releaseDate", "records[].monitored",
		"page", "pageSize", "totalRecords",
	},
	"readarr_get_cutoff": {
		"records[].id", "records[].title", "records[].authorTitle",
		"records[].releaseDate", "records[].monitored",
		"page", "pageSize", "totalRecords",
	},
	"readarr_get_queue": {
		"records[].id", "records[].title", "records[].status",
		"records[].timeleft", "records[].estimatedCompletionTime",
		"records[].protocol", "records[].downloadClient",
		"records[].bookId", "records[].authorId",
		"page", "pageSize", "totalRecords",
	},
	"readarr_get_history": {
		"records[].id", "records[].bookId", "records[].authorId",
		"records[].eventType", "records[].date",
		"records[].sourceTitle",
		"page", "pageSize", "totalRecords",
	},
	"readarr_get_history_author": {
		"id", "bookId", "authorId", "eventType", "date", "sourceTitle",
	},
	"readarr_get_history_since": {
		"id", "bookId", "authorId", "eventType", "date", "sourceTitle",
	},
	"readarr_list_commands": {
		"id", "name", "commandName", "status", "started", "ended",
		"stateChangeTime",
	},
	"readarr_get_command": {
		"id", "name", "commandName", "status", "started", "ended",
		"stateChangeTime", "body",
	},
	"readarr_list_root_folders": {
		"id", "path", "name", "accessible", "freeSpace",
	},
	"readarr_list_quality_profiles": {
		"id", "name", "cutoff.id", "cutoff.name",
	},
	"readarr_list_metadata_profiles": {
		"id", "name", "minPopularity",
	},
	"readarr_list_tags": {
		"id", "label",
	},
	"readarr_list_book_files": {
		"id", "path", "size", "quality.quality.name",
		"bookId", "authorId",
	},
	"readarr_list_blocklist": {
		"records[].id", "records[].authorId", "records[].bookIds",
		"records[].sourceTitle", "records[].date",
		"page", "pageSize", "totalRecords",
	},
	"readarr_get_rename": {
		"bookId", "bookFileId", "existingPath", "newPath",
	},
	"readarr_get_retag": {
		"bookId", "bookFileId", "path",
		"changes[].field", "changes[].oldValue", "changes[].newValue",
	},
	"readarr_get_manual_import": {
		"id", "path", "name", "size",
		"author.id", "author.authorName",
		"book.id", "book.title",
		"quality.quality.name", "rejections",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("readarr: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
