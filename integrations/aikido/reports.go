package aikido

import (
	"net/http"

	mcp "github.com/daltoniam/switchboard"
)

var reportEndpoints = []endpoint{
	{
		tool: "aikido_get_compliance_overview", method: http.MethodGet, path: "/report/{framework}/overview", scope: "reports:read", access: mcp.AccessRead,
		desc:   "Get the Aikido compliance overview for a framework (SOC 2, ISO 27001, NIS2, GDPR, CIS, CIS AWS, DORA, PCI DSS): per-control status and complying versus total rule counts. Use for audit readiness and compliance reports (paid plans).",
		params: ps(str("framework", "Compliance framework.").req().oneOf("soc2", "iso", "nis2", "gdpr", "cis", "cis_aws", "dora", "pci")),
	},
	{
		tool: "aikido_list_licenses", method: http.MethodGet, path: "/licenses", scope: "licenses:read", access: mcp.AccessRead,
		desc: "List and search the Aikido SBOM across the workspace: open source packages with license type, language, purl and the code repository or container they are found in. Use for license compliance and dependency inventory questions such as which repositories use a package.",
		params: ps(
			str("search", "Only packages whose name matches."),
			str("scope", "Dependency scope (default all).").oneOf("all", "only_dev_deps", "exclude_dev_deps"),
		),
		page: pages(10, 300, 20),
	},
	{
		tool: "aikido_overwrite_package_license", method: http.MethodPost, path: "/licenses/overwrite", scope: "licenses:write", access: mcp.AccessWrite,
		desc: "Override the detected license and risk of an open source package in Aikido, for example when the scanner misreads a license.",
		params: ps(
			str("package_name", "Package name.").req(),
			str("language", "Package language, or os for container OS packages.").req(),
			str("license_type", "Correct license, for example MIT.").req(),
			str("risk", "License risk.").req().oneOf("low", "medium", "varies_on_usage", "high", "critical"),
			str("org_name", "Publishing organization (Java packages only)."),
			str("reason", "Why the license is overridden."),
		),
	},
	{
		tool: "aikido_mark_license_internal", method: http.MethodPost, path: "/licenses/markInternal", scope: "licenses:write", access: mcp.AccessWrite,
		desc: "Mark a package as internally developed so Aikido stops flagging its license as a risk.",
		params: ps(
			str("package_name", "Package name.").req(),
			str("language", "Package language, or os for container OS packages.").req(),
			str("org_name", "Publishing organization (Java packages only)."),
		),
	},
}

var researchEndpoints = []endpoint{
	{
		tool: "aikido_get_cve", method: http.MethodGet, path: "/cve/{cve_id}", scope: "research:read", access: mcp.AccessRead,
		desc:   "Look up a CVE (or Aikido Intel AIKIDO- id) in Aikido's vulnerability database: affected package and language, severity score, TL;DR, whether it affects you, patched versions and publish date.",
		params: ps(str("cve_id", "CVE id such as CVE-2021-44228, or an AIKIDO- id.").req()),
	},
	{
		tool: "aikido_list_cwe", method: http.MethodGet, path: "/research/cwe", scope: "research:read", access: mcp.AccessRead,
		desc: "List every CWE (Common Weakness Enumeration) class Aikido tracks with its description.",
	},
	{
		tool: "aikido_get_package_health", method: http.MethodGet, path: "/research/package-health/bulk", scope: "research:read", access: mcp.AccessRead,
		desc: "Get Aikido package health scores and summaries for open source package versions before adopting or upgrading a dependency.",
		params: ps(
			str("ecosystem", "Package ecosystem.").req().oneOf("cargo", "clawhub", "cpan", "chrome", "cocoapods", "drupal_packagist", "eclipse", "edge", "firefox", "github", "github_actions", "golang", "hex", "homebrew", "huggingface", "jetbrains", "maven", "npm", "nuget", "open_vsx", "packagist", "powershell_gallery", "pub", "pypi", "ruby", "skills_sh", "snyk", "swift", "terraform", "vscode", "visual_studio", "webpage", "wordpress"),
			strs("packages", "Package versions as name@version (Maven: group:artifact@version), list or comma-separated.").req(),
		),
	},
	{
		tool: "aikido_list_malware_packages", method: http.MethodGet, path: "/research/malware/packages", scope: "research:read", access: mcp.AccessRead,
		desc: "Search Aikido Intel's feed of malicious open source packages (npm, PyPI, Maven, NuGet, VS Code extensions and more) with status, release and detection dates and reasons. Use for supply chain malware checks.",
		params: ps(
			str("search", "Package name search."),
			str("filter_ecosystem", "Only this ecosystem.").oneOf("npm", "cargo", "pypi", "packagist", "golang", "maven", "nuget", "ruby", "open_vsx", "github_act", "vscode", "chrome", "wordpress", "skills_sh"),
		),
		page: pages(10, 20, 20),
	},
	{
		tool: "aikido_list_eol_runtimes", method: http.MethodGet, path: "/research/eol/runtimes", scope: "research:read", access: mcp.AccessRead,
		desc:   "List Aikido's end-of-life (EOL) catalog of runtimes, frameworks and operating systems with EOL dates. Reference data, not what your workspace runs.",
		params: ps(str("package_name", "Only this runtime, for example nodejs or python.")),
		page:   pages(10, 100, 20),
	},
	{
		tool: "aikido_get_changelog_summary", method: http.MethodGet, path: "/changelog-summary", scope: "research:read", access: mcp.AccessRead,
		desc: "Get Aikido's AI summary of changes (breaking changes, fixes) between two versions of a package, to judge a dependency upgrade.",
		params: ps(
			str("package_name", "Package name.").req(),
			str("from_version", "Current version.").req(),
			str("to_version", "Target version.").req(),
			str("language", "Package language.").req().oneOf("JS", "PY", "GO", ".NET", "Java", "Scala", "Kotlin"),
		),
	},
}
