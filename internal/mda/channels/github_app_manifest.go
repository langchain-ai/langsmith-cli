package channels

// The github.com pages the GitHub App setup flow links to.

import "fmt"

// appOwner is where a new App is registered: an organization, or whoever is
// signed in to GitHub in the browser (personal).
type appOwner struct {
	// organization is the organization's login; "" for the personal account.
	organization string
}

// githubInstallURL is the page where an App's installer picks repositories.
func githubInstallURL(slug string) string {
	return fmt.Sprintf("https://github.com/apps/%s/installations/new", slug)
}

// githubAppPermissionsURL is the App's "Permissions & events" settings page.
func githubAppPermissionsURL(owner string, ownerIsOrg bool, slug string) string {
	if ownerIsOrg {
		return fmt.Sprintf("https://github.com/organizations/%s/settings/apps/%s/permissions", owner, slug)
	}
	return fmt.Sprintf("https://github.com/settings/apps/%s/permissions", slug)
}
