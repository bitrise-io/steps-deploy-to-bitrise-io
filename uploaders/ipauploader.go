package uploaders

import (
	"encoding/json"
	"fmt"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-xcode/exportoptions"
	"github.com/bitrise-tools/go-xcode/ipa"
	"github.com/bitrise-tools/go-xcode/plistutil"
	"github.com/bitrise-tools/go-xcode/provisioningprofile"
)

// DeployIPA ...
func DeployIPA(pth, buildURL, token, notifyUserGroups, notifyEmails, isEnablePublicPage string) (string, error) {
	log.Printf("analyzing ipa")

	infoPlistPth, err := ipa.UnwrapEmbeddedInfoPlist(pth)
	if err != nil {
		return "", fmt.Errorf("failed to unwrap Info.plist from ipa, error: %s", err)
	}

	infoPlistData, err := plistutil.NewPlistDataFromFile(infoPlistPth)
	if err != nil {
		return "", fmt.Errorf("failed to parse Info.plist, error: %s", err)
	}

	appTitle, _ := infoPlistData.GetString("CFBundleName")
	bundleID, _ := infoPlistData.GetString("CFBundleIdentifier")
	version, _ := infoPlistData.GetString("CFBundleShortVersionString")
	buildNumber, _ := infoPlistData.GetString("CFBundleVersion")
	minOSVersion, _ := infoPlistData.GetString("MinimumOSVersion")
	deviceFamilyList, _ := infoPlistData.GetUInt64Array("UIDeviceFamily")

	appInfo := map[string]interface{}{
		"app_title":          appTitle,
		"bundle_id":          bundleID,
		"version":            version,
		"build_number":       buildNumber,
		"min_OS_version":     minOSVersion,
		"device_family_list": deviceFamilyList,
	}

	log.Printf("  ipa infos: %v", appInfo)

	// ---

	provisioningProfilePth, err := ipa.UnwrapEmbeddedMobileProvision(pth)
	if err != nil {
		return "", fmt.Errorf("failed to unwrap embedded.mobilprovision from ipa, error: %s", err)
	}

	provisioningProfileData, err := provisioningprofile.NewPlistDataFromFile(provisioningProfilePth)
	if err != nil {
		return "", fmt.Errorf("failed to parse embedded.mobilprovision, error: %s", err)
	}

	creationDate, _ := provisioningProfileData.GetTime("CreationDate")
	expirationDate, _ := provisioningProfileData.GetTime("ExpirationDate")
	deviceUDIDList, _ := provisioningProfileData.GetStringArray("ProvisionedDevices")
	teamName, _ := provisioningProfileData.GetString("TeamName")
	profileName, _ := provisioningProfileData.GetString("Name")
	provisionsAlldevices, _ := provisioningProfileData.GetBool("ProvisionsAllDevices")
	exportMethod := provisioningprofile.GetExportMethod(provisioningProfileData)

	if exportMethod == exportoptions.MethodAppStore {
		log.Warnf("is_enable_public_page is set, but public download isn't allowed for app-store distributions")
		log.Warnf("setting is_enable_public_page to false ...")
		isEnablePublicPage = "false"
	}

	provisioningInfo := map[string]interface{}{
		"creation_date":          creationDate,
		"expire_date":            expirationDate,
		"device_UDID_list":       deviceUDIDList,
		"team_name":              teamName,
		"profile_name":           profileName,
		"provisions_all_devices": provisionsAlldevices,
		"ipa_export_method":      exportMethod,
	}

	// ---

	fileSize, err := fileSizeInBytes(pth)
	if err != nil {
		return "", fmt.Errorf("failed to get ipa size, error: %s", err)
	}

	ipaInfoMap := map[string]interface{}{
		"file_size_bytes":   fmt.Sprintf("%f", fileSize),
		"app_info":          appInfo,
		"provisioning_info": provisioningInfo,
	}

	artifactInfoBytes, err := json.Marshal(ipaInfoMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal ipa infos, error: %s", err)
	}

	// ---

	uploadURL, artifactID, err := createArtifact(buildURL, token, pth, "ios-ipa")
	if err != nil {
		return "", fmt.Errorf("failed to create ipa artifact, error: %s", err)
	}

	if err := uploadArtifact(uploadURL, pth, ""); err != nil {
		return "", fmt.Errorf("failed to upload ipa artifact, error: %s", err)
	}

	publicInstallPage, err := finishArtifact(buildURL, token, artifactID, string(artifactInfoBytes), notifyUserGroups, notifyEmails, isEnablePublicPage)
	if err != nil {
		return "", fmt.Errorf("failed to finish ipa artifact, error: %s", err)
	}

	return publicInstallPage, nil
}
