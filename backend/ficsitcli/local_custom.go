package ficsitcli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/satisfactorymodding/SatisfactoryModManager/backend/installfinders/common"
)

// AddLocalInstallation registers a user-supplied directory as a local Satisfactory install.
// If the path passes GetGameInfo validation, the detected metadata is used. Otherwise the
// path is still accepted and a default WindowsClient/Stable record is fabricated so SMM can
// proceed (intentional bypass of the auto-detection blocker).
func (f *ficsitCLI) AddLocalInstallation(path string) error {
	cleanPath := filepath.Clean(path)
	if cleanPath == "" || cleanPath == "." {
		return fmt.Errorf("invalid path")
	}

	if f.ficsitCli.Installations.GetInstallation(cleanPath) != nil {
		return fmt.Errorf("installation already exists")
	}

	l := slog.With(slog.String("task", "addLocalInstallation"), slog.String("path", cleanPath))

	platform := common.MakeLauncherPlatform(common.NativePlatform(), nil)

	var info *common.Installation
	installType, version, savedPath, err := common.GetGameInfo(cleanPath, platform)
	if err != nil {
		l.Warn("game info detection failed, registering custom install with defaults", slog.Any("error", err))
		info = &common.Installation{
			Path:       cleanPath,
			Version:    0,
			Type:       common.InstallTypeWindowsClient,
			Location:   common.LocationTypeLocal,
			Branch:     common.BranchStable,
			Launcher:   "Custom",
			LaunchPath: launchPathForCustom(cleanPath),
			SavedPath:  "",
		}
	} else {
		info = &common.Installation{
			Path:       cleanPath,
			Version:    version,
			Type:       installType,
			Location:   common.LocationTypeLocal,
			Branch:     common.BranchStable,
			Launcher:   "Custom",
			LaunchPath: launchPathForCustom(cleanPath),
			SavedPath:  savedPath,
		}
	}

	_, err = f.ficsitCli.Installations.AddInstallation(f.ficsitCli, cleanPath, f.GetFallbackProfile())
	if err != nil {
		return fmt.Errorf("failed to add installation: %w", err)
	}

	if err := f.ficsitCli.Installations.Save(); err != nil {
		l.Error("failed to save installations", slog.Any("error", err))
	}

	f.installationMetadata.Store(cleanPath, installationMetadata{
		State: InstallStateValid,
		Info:  info,
	})

	f.ensureSelectedInstallationIsValid()
	f.EmitGlobals()
	f.EmitModsChange()

	return nil
}

func (f *ficsitCLI) RemoveLocalInstallation(path string) error {
	cleanPath := filepath.Clean(path)
	meta, ok := f.installationMetadata.Load(cleanPath)
	if !ok {
		return fmt.Errorf("installation not found")
	}
	if meta.Info != nil && meta.Info.Location != common.LocationTypeLocal {
		return fmt.Errorf("installation is not local")
	}
	if err := f.ficsitCli.Installations.DeleteInstallation(cleanPath); err != nil {
		return fmt.Errorf("failed to delete installation: %w", err)
	}
	if err := f.ficsitCli.Installations.Save(); err != nil {
		slog.Error("failed to save installations", slog.Any("error", err))
	}
	f.installationMetadata.Delete(cleanPath)
	f.ensureSelectedInstallationIsValid()
	f.EmitGlobals()
	return nil
}

func launchPathForCustom(path string) []string {
	for _, exe := range []string{"FactoryGame.exe", "FactoryGameSteam.exe", "FactoryGameEGS.exe", "FactoryServer.exe", "FactoryServer.sh"} {
		candidate := filepath.Join(path, exe)
		if _, err := os.Stat(candidate); err == nil {
			return []string{candidate}
		}
	}
	return nil
}
