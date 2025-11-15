package main

// DBConfig holds the database connection details
type DBConfig struct {
	Engine   string
	Name     string
	User     string
	Password string
	Host     string
	Port     string
}

// DPromptsJobArgs holds the data for our job
type DPromptsJobArgs struct {
	Message string `json:"message"`
}

// Kind returns the job's type. This MUST match the key in RegisterWorkers
func (DPromptsJobArgs) Kind() string {
	return "dprompts-worker"
}