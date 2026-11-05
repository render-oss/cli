package sandboxcredential

type DeleteOut struct {
	Data DeleteOutData `json:"data"`
	Meta DeleteOutMeta `json:"meta"`
}

type DeleteOutData struct {
	ID             string `json:"id"`
	SandboxGroupID string `json:"sandboxGroupId"`
}

type DeleteOutMeta struct {
	Deleted bool   `json:"deleted"`
	Message string `json:"message,omitempty"`
}
