package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/rancher/csp-adapter/pkg/clients/aliyun"
	"github.com/rancher/csp-adapter/pkg/clients/k8s"
	"github.com/rancher/csp-adapter/pkg/metrics"
	"github.com/sirupsen/logrus"
)

type ALIYUN struct {
	cancel  context.CancelFunc
	aliyun  aliyun.Client
	k8s     k8s.Client
	scraper metrics.Scraper
}

func NewALIYUN(k k8s.Client) *ALIYUN {
	return &ALIYUN{
		//aliyun:  a,
		k8s: k,
		//scraper: s,
	}
}

func (m *ALIYUN) Start(ctx context.Context, errs chan<- error) {
	go m.start(ctx, errs)
}

func (m *ALIYUN) start(ctx context.Context, errs chan<- error) {
	for range ticker(ctx, managerInterval) {
		err := m.runComplianceCheck(ctx)
		if err != nil {
			updError := m.updateAdapterOutput(ctx, false, fmt.Sprintf("unable to run compliance check with error: %v", err),
				fmt.Sprintf("%s Unable to run the adapter, please check the adapter logs", statusPrefix))
			if updError != nil {
				errs <- err
			}
			errs <- err
		}
	}
	logrus.Infof("[manager] exiting")
}

func (m *ALIYUN) runComplianceCheck(ctx context.Context) error {
	//nodeCounts, err := m.scraper.ScrapeAndParse()
	//if err != nil {
	//	return fmt.Errorf("unable to determine number of active nodes: %v", err)
	//}
	//logrus.Debugf("found %d nodes from rancher metrics", nodeCounts.Total)
	//currentCheckoutInfo, err := m.getLicenseCheckoutInfo()
	//if err != nil {
	//	// not a breaking error, just means that we need to assume we have no registered entitlements
	//	logrus.Warnf("unable to get current license consumption info, will start fresh %v", err)
	//	currentCheckoutInfo = &licenseCheckoutInfo{
	//		EntitledLicenses: 0,
	//	}
	//}
	//requiredLicenses := int(math.Ceil(float64(nodeCounts.Total) / float64(nodesPerLicense)))
	//logrus.Debugf("have %d licenses checked out, need %d licenses", currentCheckoutInfo.EntitledLicenses, requiredLicenses)
	//if currentCheckoutInfo.EntitledLicenses != requiredLicenses || requiredLicenses != 0 {
	//	checked, err := m.aliyun.CheckoutRancherLicense(ctx)
	//	if err != nil || !checked {
	//		logrus.Warnf("unable to get Rancher license: %v", err)
	//	} else {
	//		logrus.Debugf("successfully checked in license")
	//		currentCheckoutInfo.EntitledLicenses = 0
	//	}
	//	availableLicenses, err := m.aliyun.GetNumberOfAvailableEntitlements(ctx)
	//	logrus.Debugf("found %d entitlements available", availableLicenses)
	//	if err != nil {
	//		logrus.Warnf("unable to determine number of available entitlements, will attempt full checkout %v", err)
	//		// if we can't verify how many licenses are available, assume that we have enough to meet our requirements
	//		availableLicenses = requiredLicenses
	//	}
	//	checkoutAmount := requiredLicenses
	//	if checkoutAmount > availableLicenses {
	//		// only checkout what we actually have available to us
	//		checkoutAmount = availableLicenses
	//	}
	//	if checkoutAmount > 0 {
	//		// it's possible that we have no licenses available - don't attempt checkout in this case
	//		token, expiration, err := m.aliyun.GetTokenAndExpireTime(ctx)
	//		if err != nil {
	//			return fmt.Errorf("unable to checkout rancher licenses %v", err)
	//		}
	//		currentCheckoutInfo.ConsumptionToken = token
	//		currentCheckoutInfo.EntitledLicenses = checkoutAmount
	//		currentCheckoutInfo.Expiry = parseExpirationTimestamp(expiration)
	//	}
	//}
	//
	//err = m.saveCheckoutInfo(currentCheckoutInfo)
	//if err != nil {
	//	logrus.Warnf("unable to save current checkout info, next run may fail with checkout/checkin")
	//}
	//
	//var statusMessage string
	//if currentCheckoutInfo.EntitledLicenses == requiredLicenses {
	//	statusMessage = fmt.Sprintf("%s Rancher server has the required amount of licenses", statusPrefix)
	//} else {
	//	statusMessage = fmt.Sprintf("%s You have exceeded your licensed node count. At least %d more license(s) are required in AWS to become compliant.",
	//		statusPrefix, requiredLicenses-currentCheckoutInfo.EntitledLicenses)
	//}
	configMessage := fmt.Sprintf("Rancher server required license(s) and was able to check out license(s)")

	statusMessage := fmt.Sprintf("%s You have exceeded your licensed node count. At least %d more license(s) are required in AWS to become compliant.",
		"aliyun csp adapter", 4)
	return m.updateAdapterOutput(ctx, false, configMessage, statusMessage)
}

func (m *ALIYUN) updateAdapterOutput(ctx context.Context, inCompliance bool, configMessage string, notificationMessage string) error {
	config := GetAliyunDefaultSupportConfig(m.k8s)
	//token, expiryTime, err := m.aliyun.GetTokenAndExpireTime(ctx)
	//if err != nil {
	//	return fmt.Errorf("unable to get rancher version: %v", err)
	//}
	config.CSP = AliyunCSPInfo{
		Name:   aliyunSupportConfigCSP,
		Token:  "adfadsfxcfasdfadfadf",
		Expiry: "2024-04-07T12:00:00Z",
	}
	//rancherVersion, err := m.k8s.GetRancherVersion()
	//if err != nil {
	//	return fmt.Errorf("unable to get rancher version: %v", err)
	//}
	//config.Product = createProductString(rancherVersion)
	info := ComplianceInfo{
		Message: configMessage,
	}
	//if inCompliance {
	//	info.Status = StatusInCompliance
	//} else {
	//	info.Status = StatusNotInCompliance
	//}
	info.Status = StatusNotInCompliance
	//config.Compliance = info
	err := m.k8s.UpdateUserNotification(inCompliance, notificationMessage)
	if err != nil {
		// don't bother marshalling the config if we can't report the error to the user
		return err
	}
	marshalled, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("unable to marshall config: %v", err)
	}
	return m.k8s.UpdateCSPConfigOutput(marshalled)
}

func (m *ALIYUN) getLicenseCheckoutInfo() (*licenseCheckoutInfo, error) {
	secret, err := m.k8s.GetConsumptionTokenSecret()
	if err != nil {
		return nil, err
	}
	token, tOk := secret.Data[tokenKey]
	licenses, lOk := secret.Data[nodeKey]
	expiry, eOk := secret.Data[expiryKey]
	if !(tOk && lOk && eOk) {
		// if we couldn't extract the token or node counts, we can't return accurate checkout info
		return nil, fmt.Errorf("couldn't license consumption info from secret")
	}
	numLicenses, err := strconv.Atoi(string(licenses))
	if err != nil {
		return nil, fmt.Errorf("unable to parse the number of nodes the license token is for %v", err)
	}
	expiryTime, err := time.Parse(time.RFC3339, string(expiry))
	if err != nil {
		return nil, fmt.Errorf("unable to parse the token's expiry time %v", err)
	}
	return &licenseCheckoutInfo{
		ConsumptionToken: string(token),
		EntitledLicenses: numLicenses,
		Expiry:           expiryTime,
	}, nil
}

// saveCheckoutInfo saves the checkoutInfo to the k8s cache. If this fails, returns an error
func (m *ALIYUN) saveCheckoutInfo(info *licenseCheckoutInfo) error {
	return m.k8s.UpdateConsumptionTokenSecret(map[string]string{
		tokenKey:  info.ConsumptionToken,
		nodeKey:   fmt.Sprintf("%d", info.EntitledLicenses),
		expiryKey: info.Expiry.Format(time.RFC3339),
	})
}
