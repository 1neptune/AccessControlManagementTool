package deployer

import (
	"access-control-tool/internal/models"
	"log"
)

// DeployResult represents the deployment operation result
// Fields:
//   Success - whether deployment succeeded
//   Message - result description
//   StepsCompleted - number of completed steps
//   TotalSteps - total number of steps
type DeployResult struct {
	Success        bool
	Message        string
	StepsCompleted int
	TotalSteps     int
}

// Deploy deploys Sangforwatchdog program on remote server
// Routes to corresponding implementation based on server OS type
// Parameters:
//   server - target server info including host, port, username, password, etc.
//   progressCallback - progress callback function for real-time deployment progress
// Returns:
//   DeployResult - deployment result
//   error - error during deployment
// Execution flow:
//   1. Determine OS type based on server.OSType
//   2. Route to corresponding deployment implementation (Linux or Windows)
//   3. Return deployment result
// Notes:
//   - Deployment executes on remote server, not local download
//   - Target server needs network access to download files
//   - Requires sufficient permissions (root for Linux, admin for Windows)
func Deploy(server *models.Server, progressCallback func(string)) (DeployResult, error) {
	log.Printf("[Deployer] Starting Sangforwatchdog deployment, server: %s (%s)", server.Name, server.OSType)

	if progressCallback != nil {
		progressCallback("Starting Sangforwatchdog deployment...\n")
	}

	switch server.OSType {
	case "linux":
		return deployLinux(server, progressCallback)
	case "windows":
		return deployWindows(server, progressCallback)
	default:
		log.Printf("[Deployer] Unsupported OS type: %s", server.OSType)
		if progressCallback != nil {
			progressCallback("❌ Unsupported OS type: " + server.OSType + "\n")
		}
		return DeployResult{
			Success:        false,
			Message:        "Unsupported OS type",
			StepsCompleted: 0,
			TotalSteps:     1,
		}, nil
	}
}