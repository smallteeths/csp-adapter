package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rancher/csp-adapter/pkg/clients/aws"
	"github.com/rancher/csp-adapter/pkg/clients/k8s"
	"github.com/rancher/csp-adapter/pkg/manager"
	"github.com/rancher/csp-adapter/pkg/metrics"
	"github.com/rancher/wrangler/pkg/k8scheck"
	"github.com/rancher/wrangler/pkg/ratelimit"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

var (
	Version   = "dev"
	GitCommit = "HEAD"
)

func main() {
	if err := run(); err != nil {
		logrus.Fatalf("csp-adapter failed to run with error: %v", err)
	}
}

const (
	debugEnv   = "CATTLE_DEBUG"
	devModeEnv = "CATTLE_DEV_MODE"
	driver     = "CATTLE_DRIVER"
	awsCSP     = "aws"
	aliyunCSP  = "aliyun"
)

func run() error {
	if os.Getenv(debugEnv) == "true" {
		logrus.SetLevel(logrus.DebugLevel)
	}

	logrus.Infof("csp-adapter version %s is starting", fmt.Sprintf("%s (%s)", Version, GitCommit))

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	cfg.RateLimiter = ratelimit.None
	ctx := signals.SetupSignalContext()
	err = k8scheck.Wait(ctx, *cfg)
	if err != nil {
		return err
	}

	k8sClients, err := k8s.New(ctx, cfg)
	if err != nil {
		return err
	}

	devMode := os.Getenv(devModeEnv) == "true"
	driver := os.Getenv(driver)

	switch driver {
	case awsCSP:
		return startAWS(ctx, k8sClients, cfg, devMode)
	case aliyunCSP:
		return startAliyun(ctx, k8sClients, cfg)
	default:
		return fmt.Errorf("unknown driver: %s", driver)
	}

	return nil
}

func startAWS(ctx context.Context, k8sClients *k8s.Clients, cfg *rest.Config, devMode bool) error {
	awsClient, err := aws.NewClient(ctx, devMode)
	if err != nil {
		registerErr := registerStartupError(k8sClients, createCSPInfo(awsCSP, "unknown"), err)
		if registerErr != nil {
			return fmt.Errorf("unable to start or register manager error, start error: %v, register error: %v", err, registerErr)
		}
		return fmt.Errorf("failed to start, unable to start aws client: %v", err)
	}

	hostname, err := k8sClients.GetRancherHostname()
	if err != nil {
		registerErr := registerStartupError(k8sClients, createCSPInfo(awsCSP, awsClient.AccountNumber()), err)
		if registerErr != nil {
			return fmt.Errorf("unable to start or register manager error, start error: %v, register error: %v", err, registerErr)
		}
		return fmt.Errorf("failed to start, unable to get hostname: %v", err)
	}

	m := manager.NewAWS(awsClient, k8sClients, metrics.NewScraper(hostname, cfg))

	errs := make(chan error, 1)
	m.Start(ctx, errs)
	go func() {
		for err := range errs {
			logrus.Errorf("aws manager error: %v", err)
		}
	}()
	<-ctx.Done()
	return nil
}

func startAliyun(ctx context.Context, k8sClients *k8s.Clients, cfg *rest.Config) error {
	//aliyunClient, err := aliyun.NewClient()
	//if err != nil {
	//	registerErr := registerStartupError(k8sClients, createCSPInfo(aliyunCSP, "unknown"), err)
	//	if registerErr != nil {
	//		return fmt.Errorf("unable to start or register manager error, start error: %v, register error: %v", err, registerErr)
	//	}
	//	return fmt.Errorf("failed to start, unable to start aliyun client: %v", err)
	//}

	//hostname, err := k8sClients.GetRancherHostname()
	//if err != nil {
	//	registerErr := registerStartupAliyunError(k8sClients, createAliyunCSPInfo(aliyunCSP), err)
	//	if registerErr != nil {
	//		return fmt.Errorf("unable to start or register manager error, start error: %v, register error: %v", err, registerErr)
	//	}
	//	return fmt.Errorf("failed to start, unable to get hostname: %v", err)
	//}

	m := manager.NewALIYUN(k8sClients)

	errs := make(chan error, 1)
	m.Start(ctx, errs)
	go func() {
		for err := range errs {
			logrus.Errorf("aws manager error: %v", err)
		}
	}()
	<-ctx.Done()
	return nil
}

// createCSPInfo creates a manager.CSPInfo from a provided csp name and account number
func createCSPInfo(csp, acctNumber string) manager.CSPInfo {
	return manager.CSPInfo{
		Name:       csp,
		AcctNumber: acctNumber,
	}
}

// registerStartupError registers that an error occurred when starting the manager for the cloud account represented by
// cspInfo if we could start our k8s clients but couldn't init some other part of the manager infra, we need to
// report this to the user and save the error so it can be included in the supportconfig bundle
func registerStartupError(clients *k8s.Clients, cspInfo manager.CSPInfo, startupErr error) error {
	defaultConfig := manager.GetDefaultSupportConfig(clients)
	defaultConfig.Compliance = manager.ComplianceInfo{
		Status:  manager.StatusNotInCompliance,
		Message: fmt.Sprintf("CSP adapter unable to start due to error: %v", startupErr),
	}
	defaultConfig.CSP = cspInfo
	marshalledConfig, err := json.Marshal(defaultConfig)
	if err != nil {
		return err
	}
	err = clients.UpdateUserNotification(false, "Marketplace Adapter: unable to start csp adapter, check adapter logs")
	if err != nil {
		return err
	}
	err = clients.UpdateCSPConfigOutput(marshalledConfig)
	return err
}

func createAliyunCSPInfo(csp string) manager.AliyunCSPInfo {
	return manager.AliyunCSPInfo{
		Name: csp,
	}
}

func registerStartupAliyunError(clients *k8s.Clients, cspInfo manager.AliyunCSPInfo, startupErr error) error {
	defaultConfig := manager.GetAliyunDefaultSupportConfig(clients)
	defaultConfig.Compliance = manager.ComplianceInfo{
		Status:  manager.StatusNotInCompliance,
		Message: fmt.Sprintf("CSP adapter unable to start due to error: %v", startupErr),
	}
	defaultConfig.CSP = cspInfo
	marshalledConfig, err := json.Marshal(defaultConfig)
	if err != nil {
		return err
	}
	err = clients.UpdateUserNotification(false, "Marketplace Adapter: unable to start csp adapter, check adapter logs")
	if err != nil {
		return err
	}
	err = clients.UpdateCSPConfigOutput(marshalledConfig)
	return err
}
