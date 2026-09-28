package langgraphapi

import (
	"net/url"
	"slices"
	"strings"
)

const (
	cloudControlPlaneURL  = "https://api.host.langchain.com"
	cloudDashboardURL     = "https://smith.langchain.com"
	cloudDomain           = "langchain.com"
	cloudAPIHost          = "api.smith.langchain.com"
	cloudControlPlaneHost = "api.host.langchain.com"
	cloudDashboardHost    = "smith.langchain.com"
	controlPlanePath      = "/api-host"
)

var (
	langsmithAPIPaths = []string{"/api/v1", "/api"}
	localHostnames    = []string{"localhost", "127.0.0.1"}
)

// Endpoints locates the deployment control plane and the LangSmith UI that links to it.
type Endpoints struct {
	ControlPlaneURL string
	DashboardURL    string
}

// ResolveEndpoints prefers an explicit control-plane URL, then derives one from the LangSmith API URL.
func ResolveEndpoints(hostURL, langsmithURL string) Endpoints {
	switch {
	case hostURL != "":
		return fromControlPlaneURL(hostURL)
	case langsmithURL != "":
		return fromLangSmithURL(langsmithURL)
	default:
		return Endpoints{ControlPlaneURL: cloudControlPlaneURL, DashboardURL: cloudDashboardURL}
	}
}

// IsCloud reports whether the control plane is LangSmith Cloud.
func (e Endpoints) IsCloud() bool {
	host := hostname(e.ControlPlaneURL)
	return host == cloudControlPlaneHost || strings.HasSuffix(host, "."+cloudControlPlaneHost)
}

func fromControlPlaneURL(raw string) Endpoints {
	controlPlane := strings.TrimRight(raw, "/")
	host := hostname(controlPlane)
	switch {
	case strings.HasSuffix(controlPlane, controlPlanePath):
		return Endpoints{controlPlane, strings.TrimSuffix(controlPlane, controlPlanePath)}
	case slices.Contains(localHostnames, host):
		return Endpoints{controlPlane, controlPlane}
	default:
		return Endpoints{controlPlane, cloudDashboardFor(host)}
	}
}

func fromLangSmithURL(raw string) Endpoints {
	u, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil {
		return ResolveEndpoints("", "")
	}
	host := u.Hostname()
	if isCloudHost(host) {
		return fromControlPlaneURL("https://" + cloudControlPlaneHostFor(host))
	}
	root := u.Scheme + "://" + u.Host + withoutAPIPath(u.Path)
	return Endpoints{root + controlPlanePath, root}
}

func hostname(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func isCloudHost(host string) bool {
	return host == cloudDomain || strings.HasSuffix(host, "."+cloudDomain)
}

func cloudControlPlaneHostFor(apiHost string) string {
	if region, ok := strings.CutSuffix(apiHost, "."+cloudAPIHost); ok {
		return region + "." + cloudControlPlaneHost
	}
	return cloudControlPlaneHost
}

func cloudDashboardFor(controlPlaneHost string) string {
	if region, ok := strings.CutSuffix(controlPlaneHost, "."+cloudControlPlaneHost); ok {
		return "https://" + region + "." + cloudDashboardHost
	}
	return cloudDashboardURL
}

func withoutAPIPath(path string) string {
	for _, apiPath := range langsmithAPIPaths {
		if trimmed, ok := strings.CutSuffix(path, apiPath); ok {
			return trimmed
		}
	}
	return path
}
