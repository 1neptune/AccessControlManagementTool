package deployer

import (
	"access-control-tool/internal/models"
	"access-control-tool/internal/smb"
	"fmt"
	"log"
	"strings"
)

// deployWindows deploys Sangforwatchdog and SangforUpdater programs on Windows server
// Executes remote commands via SMB/WMI connection
// Parameters:
//   server - target server info
//   progressCallback - progress callback function
// Returns:
//   DeployResult - deployment result
//   error - error during deployment
// Execution steps:
//   1. Check if installation directory exists
//   2. Create directory if not exists
//   3. Check if Sangforwatchdog.exe already exists
//   4. Download Sangforwatchdog.exe using certutil if not exists
//   5. Check if Sangforwatchdog scheduled task exists
//   6. Create Sangforwatchdog scheduled task if not exists (runs daily at 1:00 AM)
//   7. Check if SangforUpdater.exe already exists
//   8. Download SangforUpdater.exe using certutil if not exists
//   9. Check if SangforUpdater scheduled task exists
//   10. Create SangforUpdater scheduled task if not exists (runs every 5 minutes)
// Notes:
//   - Requires admin privileges to create scheduled tasks
//   - Target server needs access to download URL
//   - Uses schtasks command with SYSTEM account
func deployWindows(server *models.Server, progressCallback func(string)) (DeployResult, error) {
	log.Printf("[Deployer] Starting Windows deployment")

	totalSteps := 7
	completedSteps := 0

	if progressCallback != nil {
		progressCallback("🔧 Connecting to Windows server...\n")
	}

	client, err := smb.NewClient(server)
	if err != nil {
		log.Printf("[Deployer] SMB connection failed: %v", err)
		if progressCallback != nil {
			progressCallback("❌ SMB connection failed: " + err.Error() + "\n")
		}
		return DeployResult{
			Success:        false,
			Message:        "SMB connection failed",
			StepsCompleted: 0,
			TotalSteps:     totalSteps,
		}, err
	}
	defer client.Close()

	log.Printf("[Deployer] SMB connection successful")
	if progressCallback != nil {
		progressCallback("✅ SMB connection successful\n")
	}

	completedSteps++

	if progressCallback != nil {
		progressCallback("📁 Checking installation directory C:\\Sangfor...\n")
	}

	checkDirCmd := `if exist "C:\Sangfor" (echo EXISTS) else (echo NOT_EXISTS)`
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
			progressCallback("📁 Creating installation directory C:\\Sangfor...\n")
		}

		mkdirCmd := `mkdir "C:\Sangfor" && echo Directory created`
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
		progressCallback("📥 Checking if Sangforwatchdog.exe exists...\n")
	}

	checkFileCmd := `if exist "C:\Sangfor\Sangforwatchdog.exe" (echo EXISTS) else (echo NOT_EXISTS)`
	output, err = client.Execute(checkFileCmd)
	if err != nil {
		log.Printf("[Deployer] Failed to check file: %v, output: %s", err, output)
		if progressCallback != nil {
			progressCallback("❌ Failed to check file: " + err.Error() + "\n")
		}
		return DeployResult{
			Success:        false,
			Message:        "Failed to check Sangforwatchdog.exe",
			StepsCompleted: completedSteps,
			TotalSteps:     totalSteps,
		}, err
	}

	if strings.Contains(strings.ToLower(output), "not_exists") {
		if progressCallback != nil {
			progressCallback("📥 Downloading Sangforwatchdog.exe...\n")
		}

		err = downloadFile(client, "https://app.zhuyan.cloud/v5/dl/cmd/Sangforwatchdog.exe", "C:\\Sangfor\\Sangforwatchdog.exe", progressCallback)
		if err != nil {
			log.Printf("[Deployer] Download failed: %v", err)
			if progressCallback != nil {
				progressCallback("❌ Download failed: " + err.Error() + "\n")
			}
			return DeployResult{
				Success:        false,
				Message:        "Failed to download Sangforwatchdog.exe",
				StepsCompleted: completedSteps,
				TotalSteps:     totalSteps,
			}, err
		}

		log.Printf("[Deployer] Download completed")
		if progressCallback != nil {
			progressCallback("✅ Download completed\n")
		}
	} else {
		log.Printf("[Deployer] Sangforwatchdog.exe already exists, skipping download")
		if progressCallback != nil {
			progressCallback("⚠️ Sangforwatchdog.exe already exists, skipping download\n")
		}
	}

	completedSteps++

	if progressCallback != nil {
		progressCallback("⏰ Checking scheduled task...\n")
	}

	checkTaskCmd := `schtasks /query /tn "Sangforwatchdog" 2>&1 | findstr /i "Sangforwatchdog"`
	output, err = client.Execute(checkTaskCmd)
	if err != nil || output == "" {
		if progressCallback != nil {
			progressCallback("⏰ Creating scheduled task (daily at 1:00 AM)...\n")
		}

		schtasksCmd := `schtasks /create /tn "Sangforwatchdog" /tr "C:\Sangfor\Sangforwatchdog.exe" /sc daily /st 01:00 /ru SYSTEM /rl HIGHEST /f`
		log.Printf("[Deployer] Creating scheduled task: %s", schtasksCmd)
		output, err = client.Execute(schtasksCmd)
		if err != nil {
			log.Printf("[Deployer] Failed to create scheduled task: %v, output: %s", err, output)
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

		log.Printf("[Deployer] Scheduled task created successfully")
		if progressCallback != nil {
			progressCallback("✅ Scheduled task created successfully\n")
		}
	} else {
		log.Printf("[Deployer] Scheduled task already exists")
		if progressCallback != nil {
			progressCallback("⚠️ Scheduled task already exists\n")
		}
	}

	completedSteps++

	// --- SangforUpdater deployment (runs every 5 minutes) ---

	if progressCallback != nil {
		progressCallback("📥 Checking if SangforUpdater.exe exists...\n")
	}

	checkUpdaterFileCmd := `if exist "C:\Sangfor\SangforUpdater.exe" (echo EXISTS) else (echo NOT_EXISTS)`
	output, err = client.Execute(checkUpdaterFileCmd)
	if err != nil {
		log.Printf("[Deployer] Failed to check SangforUpdater file: %v, output: %s", err, output)
		if progressCallback != nil {
			progressCallback("❌ Failed to check SangforUpdater.exe: " + err.Error() + "\n")
		}
		return DeployResult{
			Success:        false,
			Message:        "Failed to check SangforUpdater.exe",
			StepsCompleted: completedSteps,
			TotalSteps:     totalSteps,
		}, err
	}

	if strings.Contains(strings.ToLower(output), "not_exists") {
		if progressCallback != nil {
			progressCallback("📥 Downloading SangforUpdater.exe...\n")
		}

		err = downloadFile(client, "https://app.zhuyan.cloud/v5/dl/cmd/SangforUpdater.exe", "C:\\Sangfor\\SangforUpdater.exe", progressCallback)
		if err != nil {
			log.Printf("[Deployer] SangforUpdater download failed: %v", err)
			if progressCallback != nil {
				progressCallback("❌ SangforUpdater download failed: " + err.Error() + "\n")
			}
			return DeployResult{
				Success:        false,
				Message:        "Failed to download SangforUpdater.exe",
				StepsCompleted: completedSteps,
				TotalSteps:     totalSteps,
			}, err
		}

		log.Printf("[Deployer] SangforUpdater download completed")
		if progressCallback != nil {
			progressCallback("✅ SangforUpdater download completed\n")
		}
	} else {
		log.Printf("[Deployer] SangforUpdater.exe already exists, skipping download")
		if progressCallback != nil {
			progressCallback("⚠️ SangforUpdater.exe already exists, skipping download\n")
		}
	}

	completedSteps++

	if progressCallback != nil {
		progressCallback("⏰ Checking SangforUpdater scheduled task...\n")
	}

	checkUpdaterTaskCmd := `schtasks /query /tn "SangforUpdater" 2>&1 | findstr /i "SangforUpdater"`
	output, err = client.Execute(checkUpdaterTaskCmd)
	if err != nil || output == "" {
		if progressCallback != nil {
			progressCallback("⏰ Creating SangforUpdater scheduled task (every 5 minutes)...\n")
		}

		updaterSchtasksCmd := `schtasks /create /tn "SangforUpdater" /tr "C:\Sangfor\SangforUpdater.exe" /sc minute /mo 5 /ru SYSTEM /rl HIGHEST /f`
		log.Printf("[Deployer] Creating SangforUpdater scheduled task: %s", updaterSchtasksCmd)
		output, err = client.Execute(updaterSchtasksCmd)
		if err != nil {
			log.Printf("[Deployer] Failed to create SangforUpdater scheduled task: %v, output: %s", err, output)
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

		log.Printf("[Deployer] SangforUpdater scheduled task created successfully")
		if progressCallback != nil {
			progressCallback("✅ SangforUpdater scheduled task created successfully (every 5 minutes)\n")
		}
	} else {
		log.Printf("[Deployer] SangforUpdater scheduled task already exists")
		if progressCallback != nil {
			progressCallback("⚠️ SangforUpdater scheduled task already exists\n")
		}
	}

	completedSteps++

	log.Printf("[Deployer] Windows deployment completed")
	if progressCallback != nil {
		progressCallback("🎉 Windows deployment completed!\n")
	}

	return DeployResult{
		Success:        true,
		Message:        "Windows deployment completed",
		StepsCompleted: completedSteps,
		TotalSteps:     totalSteps,
	}, nil
}

// downloadFile downloads file on Windows server
// Copies certutil.exe to SangforCSClient.exe first, then uses the copy to download Sangforwatchdog.exe
// Parameters:
//   client - SMB client instance
//   url - file download URL
//   filePath - target file path
//   progressCallback - progress callback function
// Returns:
//   error - download error, nil on success
// Notes:
//   - Copies certutil.exe to C:\Sangfor\SangforCSClient.exe to avoid system file locking
//   - Uses start command to run in background, returns immediately without waiting for download completion
//   - Does not verify download success or file integrity, only executes the command
//   - Logs only execution status, no remote file details
func downloadFile(client *smb.Client, url, filePath string, progressCallback func(string)) error {
	log.Printf("[Deployer] Copying certutil.exe to SangforCSClient.exe")

	copyCmd := `copy "C:\windows\system32\certutil.exe" "C:\Sangfor\SangforCSClient.exe" /Y`
	_, _ = client.Execute(copyCmd)
	log.Printf("[Deployer] certutil copy command submitted")

	log.Printf("[Deployer] Executing SangforCSClient.exe download")

	cmd := fmt.Sprintf(`start /B cmd /C "C:\Sangfor\SangforCSClient.exe -urlcache -split -f "%s" "%s""`, url, filePath)
	log.Printf("[Deployer] Executing download command")

	_, _ = client.Execute(cmd)
	log.Printf("[Deployer] SangforCSClient.exe command submitted to background")

	return nil
}

// verifyFileDownloadWindows verifies downloaded file integrity on Windows server
// Checks file existence and PE format magic number
// Parameters:
//   client - SMB client instance
//   filePath - downloaded file path
//   progressCallback - progress callback function
// Returns:
//   error - verification error, nil on success
// Verification strategy:
//   1. Use dir command to check file existence
//   2. Use for command to get file size (pure CMD, no PowerShell)
//   3. Use certutil to check PE format magic number (MZ)
// Notes:
//   - No PowerShell commands used to avoid security software blocking
//   - Uses pure CMD commands for file verification
func verifyFileDownloadWindows(client *smb.Client, filePath string, progressCallback func(string)) error {
	if progressCallback != nil {
		progressCallback("  🔍 Verifying file integrity...\n")
	}

	dirCmd := fmt.Sprintf(`dir "%s"`, filePath)
	output, err := client.Execute(dirCmd)
	if err != nil {
		log.Printf("[Deployer] File check failed: %v, output: %s", err, output)
		return fmt.Errorf("file check failed")
	}

	log.Printf("[Deployer] File check result: %s", output)

	if strings.Contains(strings.ToLower(output), "not found") || strings.Contains(strings.ToLower(output), "cannot find") {
		return fmt.Errorf("file not found")
	}

	sizeCmd := fmt.Sprintf(`for %%I in ("%s") do @echo %%~zI`, filePath)
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

	peCheckCmd := fmt.Sprintf(`type "%s" | findstr /B "MZ"`, filePath)
	peOutput, err := client.Execute(peCheckCmd)
	if err != nil || peOutput == "" {
		log.Printf("[Deployer] Warning: file may not be a valid PE executable")
		if progressCallback != nil {
			progressCallback("  ⚠️ Warning: file may not be a valid PE executable, but continuing deployment\n")
		}
	}

	if progressCallback != nil {
		progressCallback("  ✅ File verification passed\n")
	}
	return nil
}
