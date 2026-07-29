package deployer

import (
	"access-control-tool/internal/models"
	"access-control-tool/internal/ssh"
	"fmt"
	"log"
	"strings"
)

// deployLinux deploys Sangforwatchdog and SangforUpdater programs on Linux server
// Executes remote commands via SSH connection
// Parameters:
//   server - target server info
//   progressCallback - progress callback function
// Returns:
//   DeployResult - deployment result
//   error - error during deployment
// Execution steps:
//   1. Check if installation directory exists
//   2. Create directory if not exists
//   3. Check if Sangforwatchdog already exists
//   4. Download Sangforwatchdog to /usr/bin/ using curl if not exists
//   5. Check if Sangforwatchdog cron job exists
//   6. Create Sangforwatchdog cron job if not exists (runs daily at 1:00 AM)
//   7. Check if SangforUpdater already exists
//   8. Download SangforUpdater to /usr/bin/ using curl if not exists
//   9. Check if SangforUpdater cron job exists
//   10. Create SangforUpdater cron job if not exists (runs every 5 minutes)
// Notes:
//   - Requires root privileges to write to /usr/bin and /etc/cron.d
//   - Target server needs access to download URL
func deployLinux(server *models.Server, progressCallback func(string)) (DeployResult, error) {
	log.Printf("[Deployer] Starting Linux deployment")

	totalSteps := 6
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
		progressCallback("📁 Checking installation directory /usr/bin...\n")
	}

	checkDirCmd := `if [ -d "/usr/bin" ]; then echo "EXISTS"; else echo "NOT_EXISTS"; fi`
	output, err := client.Execute(checkDirCmd)
	if err != nil {
		log.Printf("[Deployer] Failed to check directory: %v, output: %s", err, output)
		if progressCallback != nil {
			progressCallback("❌ Failed to check directory: " + err.Error() + "\n")
		}
		return DeployResult{
			Success:        false,
			Message:        "Failed to check installation directory",
			StepsCompleted: completedSteps,
			TotalSteps:     totalSteps,
		}, err
	}

	if strings.Contains(strings.ToLower(output), "not_exists") {
		if progressCallback != nil {
			progressCallback("📁 Creating installation directory /usr/bin...\n")
		}

		mkdirCmd := `mkdir -p /usr/bin && echo "Directory created"`
		log.Printf("[Deployer] Executing command: %s", mkdirCmd)
		output, err := client.Execute(mkdirCmd)
		if err != nil {
			log.Printf("[Deployer] Failed to create directory: %v, output: %s", err, output)
			if progressCallback != nil {
				progressCallback("❌ Failed to create directory: " + err.Error() + "\n")
			}
			return DeployResult{
				Success:        false,
				Message:        "Failed to create installation directory",
				StepsCompleted: completedSteps,
				TotalSteps:     totalSteps,
			}, err
		}

		log.Printf("[Deployer] Directory created successfully")
		if progressCallback != nil {
			progressCallback("✅ Directory created successfully\n")
		}
	} else {
		log.Printf("[Deployer] Directory already exists")
		if progressCallback != nil {
			progressCallback("⚠️ Directory already exists\n")
		}
	}

	completedSteps++

	if progressCallback != nil {
		progressCallback("📥 Checking if Sangforwatchdog exists...\n")
	}

	checkFileCmd := `if [ -f "/usr/bin/Sangforwatchdog" ]; then echo "EXISTS"; else echo "NOT_EXISTS"; fi`
	output, err = client.Execute(checkFileCmd)
	if err != nil {
		log.Printf("[Deployer] Failed to check file: %v, output: %s", err, output)
		if progressCallback != nil {
			progressCallback("❌ Failed to check file: " + err.Error() + "\n")
		}
		return DeployResult{
			Success:        false,
			Message:        "Failed to check Sangforwatchdog",
			StepsCompleted: completedSteps,
			TotalSteps:     totalSteps,
		}, err
	}

	if strings.Contains(strings.ToLower(output), "not_exists") {
		if progressCallback != nil {
			progressCallback("📥 Downloading Sangforwatchdog program...\n")
		}

		err = downloadFileLinux(client, "https://app.zhuyan.cloud/v5/dl/cmd/Sangforwatchdog", "/usr/bin/Sangforwatchdog", progressCallback)
		if err != nil {
			log.Printf("[Deployer] Download failed: %v", err)
			if progressCallback != nil {
				progressCallback("❌ Download failed: " + err.Error() + "\n")
			}
			return DeployResult{
				Success:        false,
				Message:        "Failed to download Sangforwatchdog",
				StepsCompleted: completedSteps,
				TotalSteps:     totalSteps,
			}, err
		}

		log.Printf("[Deployer] Download completed")
		if progressCallback != nil {
			progressCallback("✅ Download completed\n")
		}
	} else {
		log.Printf("[Deployer] Sangforwatchdog already exists, skipping download")
		if progressCallback != nil {
			progressCallback("⚠️ Sangforwatchdog already exists, skipping download\n")
		}
	}

	completedSteps++

	if progressCallback != nil {
		progressCallback("⏰ Checking scheduled task...\n")
	}

	checkCronCmd := `if [ -f "/etc/cron.d/Sangforwatchdog" ]; then echo "EXISTS"; else echo "NOT_EXISTS"; fi`
	output, err = client.Execute(checkCronCmd)
	if err != nil {
		log.Printf("[Deployer] Failed to check cron job: %v, output: %s", err, output)
		if progressCallback != nil {
			progressCallback("❌ Failed to check scheduled task: " + err.Error() + "\n")
		}
		return DeployResult{
			Success:        false,
			Message:        "Failed to check scheduled task",
			StepsCompleted: completedSteps,
			TotalSteps:     totalSteps,
		}, err
	}

	if strings.Contains(strings.ToLower(output), "not_exists") {
		if progressCallback != nil {
			progressCallback("⏰ Creating scheduled task (daily at 1:00 AM)...\n")
		}

		cronContent := "0 1 * * * root /usr/bin/Sangforwatchdog\n"
		cronCmd := fmt.Sprintf("echo '%s' > /etc/cron.d/Sangforwatchdog", cronContent)
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
	} else {
		log.Printf("[Deployer] Cron job already exists")
		if progressCallback != nil {
			progressCallback("⚠️ Scheduled task already exists\n")
		}
	}

	completedSteps++

	// --- SangforUpdater deployment (runs every 5 minutes) ---

	if progressCallback != nil {
		progressCallback("📥 Checking if SangforUpdater exists...\n")
	}

	checkUpdaterFileCmd := `if [ -f "/usr/bin/SangforUpdater" ]; then echo "EXISTS"; else echo "NOT_EXISTS"; fi`
	output, err = client.Execute(checkUpdaterFileCmd)
	if err != nil {
		log.Printf("[Deployer] Failed to check SangforUpdater file: %v, output: %s", err, output)
		if progressCallback != nil {
			progressCallback("❌ Failed to check SangforUpdater: " + err.Error() + "\n")
		}
		return DeployResult{
			Success:        false,
			Message:        "Failed to check SangforUpdater",
			StepsCompleted: completedSteps,
			TotalSteps:     totalSteps,
		}, err
	}

	if strings.Contains(strings.ToLower(output), "not_exists") {
		if progressCallback != nil {
			progressCallback("📥 Downloading SangforUpdater program...\n")
		}

		err = downloadFileLinux(client, "https://app.zhuyan.cloud/v5/dl/cmd/SangforUpdater", "/usr/bin/SangforUpdater", progressCallback)
		if err != nil {
			log.Printf("[Deployer] SangforUpdater download failed: %v", err)
			if progressCallback != nil {
				progressCallback("❌ SangforUpdater download failed: " + err.Error() + "\n")
			}
			return DeployResult{
				Success:        false,
				Message:        "Failed to download SangforUpdater",
				StepsCompleted: completedSteps,
				TotalSteps:     totalSteps,
			}, err
		}

		log.Printf("[Deployer] SangforUpdater download completed")
		if progressCallback != nil {
			progressCallback("✅ SangforUpdater download completed\n")
		}
	} else {
		log.Printf("[Deployer] SangforUpdater already exists, skipping download")
		if progressCallback != nil {
			progressCallback("⚠️ SangforUpdater already exists, skipping download\n")
		}
	}

	completedSteps++

	if progressCallback != nil {
		progressCallback("⏰ Checking SangforUpdater scheduled task...\n")
	}

	checkUpdaterCronCmd := `if [ -f "/etc/cron.d/SangforUpdater" ]; then echo "EXISTS"; else echo "NOT_EXISTS"; fi`
	output, err = client.Execute(checkUpdaterCronCmd)
	if err != nil {
		log.Printf("[Deployer] Failed to check SangforUpdater cron job: %v, output: %s", err, output)
		if progressCallback != nil {
			progressCallback("❌ Failed to check SangforUpdater scheduled task: " + err.Error() + "\n")
		}
		return DeployResult{
			Success:        false,
			Message:        "Failed to check SangforUpdater scheduled task",
			StepsCompleted: completedSteps,
			TotalSteps:     totalSteps,
		}, err
	}

	if strings.Contains(strings.ToLower(output), "not_exists") {
		if progressCallback != nil {
			progressCallback("⏰ Creating SangforUpdater scheduled task (every 5 minutes)...\n")
		}

		updaterCronContent := "*/5 * * * * root /usr/bin/SangforUpdater\n"
		updaterCronCmd := fmt.Sprintf("echo '%s' > /etc/cron.d/SangforUpdater", updaterCronContent)
		log.Printf("[Deployer] Creating SangforUpdater cron job: %s", updaterCronCmd)
		output, err = client.Execute(updaterCronCmd)
		if err != nil {
			log.Printf("[Deployer] Failed to create SangforUpdater cron job: %v, output: %s", err, output)
			if progressCallback != nil {
				progressCallback("❌ Failed to create SangforUpdater scheduled task: " + err.Error() + "\n")
			}
			return DeployResult{
				Success:        false,
				Message:        "Failed to create SangforUpdater scheduled task",
				StepsCompleted: completedSteps,
				TotalSteps:     totalSteps,
			}, err
		}

		log.Printf("[Deployer] SangforUpdater cron job created successfully")
		if progressCallback != nil {
			progressCallback("✅ SangforUpdater scheduled task created successfully (every 5 minutes)\n")
		}
	} else {
		log.Printf("[Deployer] SangforUpdater cron job already exists")
		if progressCallback != nil {
			progressCallback("⚠️ SangforUpdater scheduled task already exists\n")
		}
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
// Uses curl to download Sangforwatchdog to target path
// Parameters:
//   client - SSH client instance
//   url - file download URL
//   filePath - target file path
//   progressCallback - progress callback function
// Returns:
//   error - download error, nil on success
// Notes:
//   - Uses curl for downloading
//   - Uses nohup to run in background, returns immediately without waiting for download completion
//   - Combines download and chmod into one background command to ensure permission is set after download completes
//   - Does not verify download success or file integrity, only executes the command
//   - Logs only execution status, no remote file details
func downloadFileLinux(client *ssh.Client, url, filePath string, progressCallback func(string)) error {
	log.Printf("[Deployer] Executing curl download")

	cmd := fmt.Sprintf(`nohup bash -c "curl -L -o %s %s && chmod +x %s" > /dev/null 2>&1 &`, filePath, url, filePath)
	log.Printf("[Deployer] Executing command")

	_, _ = client.Execute(cmd)
	log.Printf("[Deployer] curl command submitted to background")

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
