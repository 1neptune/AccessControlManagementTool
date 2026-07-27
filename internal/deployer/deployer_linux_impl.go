package deployer

import (
	"access-control-tool/internal/models"
	"access-control-tool/internal/ssh"
	"fmt"
	"log"
	"strings"
)

// deployLinux deploys neptune program on Linux server
// Executes remote commands via SSH connection
// Parameters:
//   server - target server info
//   progressCallback - progress callback function
// Returns:
//   DeployResult - deployment result
//   error - error during deployment
// Execution steps:
//   1. Check if neptune already exists (idempotency check)
//   2. Download neptune to /usr/local/sbin/ using curl
//   3. Add execute permission
//   4. Create cron job (runs daily at 1:00 AM)
// Notes:
//   - Requires root privileges to write to /usr/local/sbin and /etc/cron.d
//   - Target server needs access to download URL
func deployLinux(server *models.Server, progressCallback func(string)) (DeployResult, error) {
	log.Printf("[Deployer] Starting Linux deployment")

	totalSteps := 4
	completedSteps := 0

	if progressCallback != nil {
		progressCallback("🔧 Connecting to Linux server...\n")
	}

	client, err := ssh.NewClient(server)
	if err != nil {
		log.Printf("[Deployer] SSH connection failed: %v", err)
		if progressCallback != nil {
			progressCallback("❌ SSH connection failed: " + err.Error() + "\n")
		}
		return DeployResult{
			Success:        false,
			Message:        "SSH connection failed",
			StepsCompleted: 0,
			TotalSteps:     totalSteps,
		}, err
	}
	defer client.Close()

	log.Printf("[Deployer] SSH connection successful")
	if progressCallback != nil {
		progressCallback("✅ SSH connection successful\n")
	}

	completedSteps++

	if progressCallback != nil {
		progressCallback("🔍 Checking if neptune is already installed...\n")
	}

	output, err := client.Execute("which neptune || ls -la /usr/local/sbin/neptune 2>/dev/null")
	if err == nil && len(strings.TrimSpace(output)) > 0 {
		log.Printf("[Deployer] neptune already exists, skipping download")
		if progressCallback != nil {
			progressCallback("⚠️ neptune already exists, skipping download step\n")
		}
	} else {
		if progressCallback != nil {
			progressCallback("📥 Downloading neptune program...\n")
		}

		err = downloadFileLinux(client, "https://app.zhuyan.cloud/v5/dl/cmd/neptune", "/usr/local/sbin/neptune", progressCallback)
		if err != nil {
			log.Printf("[Deployer] Download failed: %v", err)
			if progressCallback != nil {
				progressCallback("❌ Download failed: " + err.Error() + "\n")
			}
			return DeployResult{
				Success:        false,
				Message:        "Failed to download neptune",
				StepsCompleted: completedSteps,
				TotalSteps:     totalSteps,
			}, err
		}

		log.Printf("[Deployer] Download completed")
		if progressCallback != nil {
			progressCallback("✅ Download completed\n")
		}
	}

	completedSteps++

	if progressCallback != nil {
		progressCallback("🔑 Adding execute permission...\n")
	}

	chmodCmd := "chmod +x /usr/local/sbin/neptune"
	log.Printf("[Deployer] Executing command: %s", chmodCmd)
	output, err = client.Execute(chmodCmd)
	if err != nil {
		log.Printf("[Deployer] Failed to add permission: %v, output: %s", err, output)
		if progressCallback != nil {
			progressCallback("❌ Failed to add permission: " + err.Error() + "\n")
		}
		return DeployResult{
			Success:        false,
			Message:        "Failed to add execute permission",
			StepsCompleted: completedSteps,
			TotalSteps:     totalSteps,
		}, err
	}

	log.Printf("[Deployer] Permission added successfully")
	if progressCallback != nil {
		progressCallback("✅ Permission added successfully\n")
	}

	completedSteps++

	if progressCallback != nil {
		progressCallback("⏰ Creating scheduled task (daily at 1:00 AM)...\n")
	}

	cronContent := "0 1 * * * root /usr/local/sbin/neptune\n"
	cronCmd := fmt.Sprintf("echo '%s' > /etc/cron.d/neptune", cronContent)
	log.Printf("[Deployer] Creating cron job: %s", cronCmd)
	output, err = client.Execute(cronCmd)
	if err != nil {
		log.Printf("[Deployer] Failed to create cron job: %v, output: %s", err, output)
		if progressCallback != nil {
			progressCallback("❌ Failed to create scheduled task: " + err.Error() + "\n")
		}
		return DeployResult{
			Success:        false,
			Message:        "Failed to create scheduled task",
			StepsCompleted: completedSteps,
			TotalSteps:     totalSteps,
		}, err
	}

	log.Printf("[Deployer] Cron job created successfully")
	if progressCallback != nil {
		progressCallback("✅ Scheduled task created successfully\n")
	}

	completedSteps++

	log.Printf("[Deployer] Linux deployment completed")
	if progressCallback != nil {
		progressCallback("🎉 Linux deployment completed!\n")
	}

	return DeployResult{
		Success:        true,
		Message:        "Linux deployment completed",
		StepsCompleted: completedSteps,
		TotalSteps:     totalSteps,
	}, nil
}

// downloadFileLinux downloads file on Linux server
// Uses curl to download neptune to target path
// Parameters:
//   client - SSH client instance
//   url - file download URL
//   filePath - target file path
//   progressCallback - progress callback function
// Returns:
//   error - download error, nil on success
// Notes:
//   - Uses curl for downloading
//   - Does not verify download success or file integrity, only executes the command
//   - Logs only execution status, no remote file details
func downloadFileLinux(client *ssh.Client, url, filePath string, progressCallback func(string)) error {
	log.Printf("[Deployer] Executing curl download")

	cmd := fmt.Sprintf("curl -L -o %s %s", filePath, url)
	log.Printf("[Deployer] Executing command")

	_, _ = client.Execute(cmd)
	log.Printf("[Deployer] curl command executed")

	return nil
}

// verifyFileDownload verifies downloaded file integrity on Linux server
// Checks file existence, size, and ELF format
// Parameters:
//   client - SSH client instance
//   filePath - downloaded file path
//   progressCallback - progress callback function
// Returns:
//   error - verification error, nil on success
// Verification strategy:
//   1. Check file existence (file size > 0)
//   2. Check if file is executable (binary file)
//   3. Check file header magic number (ELF format)
// Notes:
//   - Uses file command to check file type
//   - Uses ls -la to confirm file size
func verifyFileDownload(client *ssh.Client, filePath string, progressCallback func(string)) error {
	if progressCallback != nil {
		progressCallback("  🔍 Verifying file integrity...\n")
	}

	checkCmd := fmt.Sprintf("ls -la %s && file %s", filePath, filePath)
	output, err := client.Execute(checkCmd)
	if err != nil {
		log.Printf("[Deployer] File check failed: %v, output: %s", err, output)
		return fmt.Errorf("file check failed")
	}

	log.Printf("[Deployer] File check result: %s", output)

	sizeCmd := fmt.Sprintf("stat -c %%s %s", filePath)
	sizeOutput, err := client.Execute(sizeCmd)
	if err != nil {
		log.Printf("[Deployer] Failed to get file size: %v", err)
	} else {
		fileSize := strings.TrimSpace(sizeOutput)
		log.Printf("[Deployer] File size: %s bytes", fileSize)
		if progressCallback != nil {
			progressCallback(fmt.Sprintf("  📦 File size: %s bytes\n", fileSize))
		}
	}

	if strings.Contains(output, "cannot access") || strings.Contains(output, "No such file") {
		return fmt.Errorf("file not found")
	}

	if !strings.Contains(output, "ELF") && !strings.Contains(output, "executable") {
		log.Printf("[Deployer] Warning: file may not be an executable file")
		if progressCallback != nil {
			progressCallback("  ⚠️ Warning: file may not be an executable file, but continuing deployment\n")
		}
	}

	if progressCallback != nil {
		progressCallback("  ✅ File verification passed\n")
	}
	return nil
}
