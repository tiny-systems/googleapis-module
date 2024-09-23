package etc

type ClientConfig struct {
	Credentials string   `json:"credentials" required:"true" format:"textarea" title:"Credentials" description:"Google client credentials.json file content"`
	Scopes      []string `json:"scopes,omitempty" title:"Scopes"`
}
