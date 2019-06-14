package uploaders

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/bitrise-io/go-android/sdk"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
)

// ApkInfo ...
type ApkInfo struct {
	AppName           string
	PackageName       string
	VersionCode       string
	VersionName       string
	MinSDKVersion     string
	RawPackageContent string
}

func packageField(data, key string) string {
	pattern := fmt.Sprintf(`%s=['"](.*?)['"]`, key)

	re := regexp.MustCompile(pattern)
	if matches := re.FindStringSubmatch(data); len(matches) == 2 {
		return matches[1]
	}

	return ""
}

func filterPackageInfos(aaptOut string) (string, string, string) {
	return packageField(aaptOut, "name"),
		packageField(aaptOut, "versionCode"),
		packageField(aaptOut, "versionName")
}

func filterAppLable(aaptOut string) string {
	pattern := `application: label=\'(?P<label>.+)\' icon=`
	re := regexp.MustCompile(pattern)
	if matches := re.FindStringSubmatch(aaptOut); len(matches) == 2 {
		return matches[1]
	}

	pattern = `application-label:\'(?P<label>.*)\'`
	re = regexp.MustCompile(pattern)
	if matches := re.FindStringSubmatch(aaptOut); len(matches) == 2 {
		return matches[1]
	}

	return ""
}

func filterMinSDKVersion(aaptOut string) string {
	pattern := `sdkVersion:\'(?P<min_sdk_version>.*)\'`
	re := regexp.MustCompile(pattern)
	if matches := re.FindStringSubmatch(aaptOut); len(matches) == 2 {
		return matches[1]
	}
	return ""
}

func getAPKInfo(apkPth string) (ApkInfo, error) {
	androidHome := os.Getenv("ANDROID_HOME")
	if androidHome == "" {
		return ApkInfo{}, errors.New("ANDROID_HOME environment not set")
	}

	sdkModel, err := sdk.New(androidHome)
	if err != nil {
		return ApkInfo{}, fmt.Errorf("failed to create sdk model, error: %s", err)
	}

	aaptPth, err := sdkModel.LatestBuildToolPath("aapt")
	if err != nil {
		return ApkInfo{}, fmt.Errorf("failed to find latest aapt binary, error: %s", err)
	}

	aaptOut, err := command.New(aaptPth, "dump", "badging", apkPth).RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return ApkInfo{}, fmt.Errorf("failed to get apk infos, output: %s, error: %s", aaptOut, err)
	}

	appName := filterAppLable(aaptOut)
	packageName, versionCode, versionName := filterPackageInfos(aaptOut)
	minSDKVersion := filterMinSDKVersion(aaptOut)

	packageContent := ""
	for _, line := range strings.Split(aaptOut, "\n") {
		if strings.HasPrefix(line, "package:") {
			packageContent = line
		}
	}

	return ApkInfo{
		AppName:           appName,
		PackageName:       packageName,
		VersionCode:       versionCode,
		VersionName:       versionName,
		MinSDKVersion:     minSDKVersion,
		RawPackageContent: packageContent,
	}, nil
}

// DeployAPK ...
func DeployAPK(pth, buildURL, token, notifyUserGroups, notifyEmails, isEnablePublicPage string) (string, error) {
	log.Printf("analyzing apk")

	apkInfo, err := getAPKInfo(pth)
	if err != nil {
		return "", err
	}

	appInfo := map[string]interface{}{
		"app_name":        apkInfo.AppName,
		"package_name":    apkInfo.PackageName,
		"version_code":    apkInfo.VersionCode,
		"version_name":    apkInfo.VersionName,
		"min_sdk_version": apkInfo.MinSDKVersion,
	}

	log.Printf("  apk infos: %v", appInfo)

	if apkInfo.PackageName == "" {
		log.Warnf("Package name is undefined, AndroidManifest.xml package content:\n%s", apkInfo.RawPackageContent)
	}

	if apkInfo.VersionCode == "" {
		log.Warnf("Version code is undefined, AndroidManifest.xml package content:\n%s", apkInfo.RawPackageContent)
	}

	if apkInfo.VersionName == "" {
		log.Warnf("Version name is undefined, AndroidManifest.xml package content:\n%s", apkInfo.RawPackageContent)
	}

	// ---

	fileSize, err := fileSizeInBytes(pth)
	if err != nil {
		return "", fmt.Errorf("failed to get apk size, error: %s", err)
	}

	apkInfoMap := map[string]interface{}{
		"file_size_bytes": fmt.Sprintf("%f", fileSize),
		"app_info":        appInfo,
	}

	artifactInfoBytes, err := json.Marshal(apkInfoMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal apk infos, error: %s", err)
	}

	// ---

	uploadURL, artifactID, err := createArtifact(buildURL, token, pth, "android-apk", "")
	if err != nil {
		return "", fmt.Errorf("failed to create apk artifact, error: %s", err)
	}

	if err := uploadArtifact(uploadURL, pth, "application/vnd.android.package-archive"); err != nil {
		return "", fmt.Errorf("failed to upload apk artifact, error: %s", err)
	}

	publicInstallPage, err := finishArtifact(buildURL, token, artifactID, string(artifactInfoBytes), notifyUserGroups, notifyEmails, isEnablePublicPage)
	if err != nil {
		return "", fmt.Errorf("failed to finish apk artifact, error: %s", err)
	}

	return publicInstallPage, nil
}
