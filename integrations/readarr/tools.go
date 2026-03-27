package readarr

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Books ───────────────────────────────────────────────────────
	{
		Name: "readarr_list_books", Description: "List all books in the Readarr library. Start here for book management, reading list tracking, and ebook/audiobook collection overview.",
		Parameters: map[string]string{"author_id": "Filter by author ID (integer)", "include_all_author_books": "Include all books by the author (true/false)"},
	},
	{
		Name: "readarr_get_book", Description: "Get details of a specific book by ID, including editions, files, author info. Use after list_books or search_books.",
		Parameters: map[string]string{"id": "Book ID (integer)"},
		Required:   []string{"id"},
	},
	{
		Name: "readarr_search_books", Description: "Search for books and authors by title, author name, or ISBN. Use to find new books to add to Readarr library. Returns search results from metadata providers.",
		Parameters: map[string]string{"term": "Search query — book title, author name, or ISBN"},
		Required:   []string{"term"},
	},
	{
		Name: "readarr_monitor_books", Description: "Bulk update monitoring status for books. Set which books Readarr should actively search for downloads.",
		Parameters: map[string]string{"book_ids": "Comma-separated list of book IDs to update", "monitored": "Whether to monitor the books (true/false)"},
		Required:   []string{"book_ids", "monitored"},
	},

	// ── Authors ─────────────────────────────────────────────────────
	{
		Name: "readarr_list_authors", Description: "List all authors in the Readarr library. Shows monitored authors, book counts, and quality profiles.",
		Parameters: map[string]string{},
	},
	{
		Name: "readarr_get_author", Description: "Get details of a specific author by ID, including books, statistics, and metadata. Use after list_authors.",
		Parameters: map[string]string{"id": "Author ID (integer)"},
		Required:   []string{"id"},
	},

	// ── Calendar ────────────────────────────────────────────────────
	{
		Name: "readarr_get_calendar", Description: "Get upcoming and recent book releases within a date range. Shows release calendar for monitored books and authors.",
		Parameters: map[string]string{"start": "Start date (ISO 8601, e.g. 2024-01-01)", "end": "End date (ISO 8601)", "unmonitored": "Include unmonitored books (true/false, default false)", "include_author": "Include author data (true/false)"},
	},

	// ── Wanted / Missing ────────────────────────────────────────────
	{
		Name: "readarr_get_missing", Description: "List books that are monitored but missing from the library. Shows wanted books that Readarr is searching for. Paginated.",
		Parameters: map[string]string{"page": "Page number (default 1)", "page_size": "Items per page (default 20)", "sort_key": "Sort field (e.g. title, releaseDate)", "sort_direction": "Sort direction (asc/desc)"},
	},
	{
		Name: "readarr_get_cutoff", Description: "List books that have files but don't meet quality cutoff. Shows books eligible for quality upgrades.",
		Parameters: map[string]string{"page": "Page number (default 1)", "page_size": "Items per page (default 20)", "sort_key": "Sort field", "sort_direction": "Sort direction (asc/desc)"},
	},

	// ── Queue ───────────────────────────────────────────────────────
	{
		Name: "readarr_get_queue", Description: "Get the current download queue. Shows books being downloaded, their progress, status, and download client info. Paginated.",
		Parameters: map[string]string{"page": "Page number (default 1)", "page_size": "Items per page (default 20)", "sort_key": "Sort field (e.g. timeleft, title)", "sort_direction": "Sort direction (asc/desc)", "include_author": "Include author data (true/false)", "include_book": "Include book data (true/false)"},
	},
	{
		Name: "readarr_delete_queue_item", Description: "Remove an item from the download queue. Can optionally blocklist and remove from download client.",
		Parameters: map[string]string{"id": "Queue item ID (integer)", "remove_from_client": "Remove from download client (true/false, default true)", "blocklist": "Add release to blocklist (true/false, default false)"},
		Required:   []string{"id"},
	},
	{
		Name: "readarr_delete_queue_bulk", Description: "Bulk remove items from the download queue.",
		Parameters: map[string]string{"ids": "Comma-separated queue item IDs to remove", "remove_from_client": "Remove from download client (true/false, default true)", "blocklist": "Add releases to blocklist (true/false, default false)"},
		Required:   []string{"ids"},
	},
	{
		Name: "readarr_grab_queue_item", Description: "Force grab a pending queue item to begin download immediately.",
		Parameters: map[string]string{"id": "Queue item ID (integer)"},
		Required:   []string{"id"},
	},

	// ── History ─────────────────────────────────────────────────────
	{
		Name: "readarr_get_history", Description: "Get download history for books. Shows grabbed, imported, failed, and other download events. Paginated.",
		Parameters: map[string]string{"page": "Page number (default 1)", "page_size": "Items per page (default 20)", "sort_key": "Sort field (e.g. date)", "sort_direction": "Sort direction (asc/desc)", "event_type": "Filter by event type (grabbed, bookFileImported, downloadFailed, etc.)", "book_id": "Filter by book ID (integer)", "include_author": "Include author data (true/false)", "include_book": "Include book data (true/false)"},
	},
	{
		Name: "readarr_get_history_author", Description: "Get download history for a specific author. Use after list_authors or get_author.",
		Parameters: map[string]string{"author_id": "Author ID (integer)"},
		Required:   []string{"author_id"},
	},
	{
		Name: "readarr_get_history_since", Description: "Get download history since a specific date.",
		Parameters: map[string]string{"date": "ISO 8601 date to get history since", "event_type": "Filter by event type"},
		Required:   []string{"date"},
	},

	// ── Commands ────────────────────────────────────────────────────
	{
		Name: "readarr_list_commands", Description: "List all running and recently completed commands. Shows refresh, scan, and search task status.",
		Parameters: map[string]string{},
	},
	{
		Name: "readarr_run_command", Description: "Execute a Readarr command: RefreshBook, RefreshAuthor, RescanFolders, MissingBookSearch, AuthorSearch, BookSearch, RenameAuthor, RenameFiles, RetagAuthor, RetagFiles.",
		Parameters: map[string]string{"name": "Command name (e.g. RefreshBook, MissingBookSearch, AuthorSearch)", "author_id": "Author ID for author-specific commands (integer)", "book_id": "Book ID for book-specific commands (integer)"},
		Required:   []string{"name"},
	},
	{
		Name: "readarr_get_command", Description: "Get the status of a specific command by ID. Use after run_command to check progress.",
		Parameters: map[string]string{"id": "Command ID (integer)"},
		Required:   []string{"id"},
	},

	// ── System ──────────────────────────────────────────────────────
	{
		Name: "readarr_get_system_status", Description: "Get Readarr system status including version, OS, runtime, and startup info. Use to verify connectivity and check server health.",
		Parameters: map[string]string{},
	},

	// ── Root Folders ────────────────────────────────────────────────
	{
		Name: "readarr_list_root_folders", Description: "List configured root folders where Readarr stores book files. Shows paths, free space, and unmapped folders.",
		Parameters: map[string]string{},
	},

	// ── Quality Profiles ────────────────────────────────────────────
	{
		Name: "readarr_list_quality_profiles", Description: "List quality profiles that control download format preferences (EPUB, MOBI, AZW3, PDF, audiobook). Used when adding books or authors.",
		Parameters: map[string]string{},
	},

	// ── Metadata Profiles ───────────────────────────────────────────
	{
		Name: "readarr_list_metadata_profiles", Description: "List metadata profiles that control which book editions and metadata sources Readarr uses.",
		Parameters: map[string]string{},
	},

	// ── Tags ────────────────────────────────────────────────────────
	{
		Name: "readarr_list_tags", Description: "List all tags used to organize and filter books and authors in Readarr.",
		Parameters: map[string]string{},
	},
}
