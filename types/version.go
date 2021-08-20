package types

type Version struct {
	Version                  string `json:"version"`
	GrindVersionRequired     string `json:"grindVersionRequired"`
	GrindVersionRecommended  string `json:"grindVersionRecommended"`
	ThonnyVersionRequired    string `json:"thonnyVersionRequired"`
	ThonnyVersionRecommended string `json:"thonnyVersionRecommended"`
}

var CurrentVersion = Version{
	Version:                  "2.6.2",
	GrindVersionRequired:     "2.6.2",
	GrindVersionRecommended:  "2.6.2",
	ThonnyVersionRecommended: "2.6.9",
	ThonnyVersionRequired:    "2.6.9",
}
