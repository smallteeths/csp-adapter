package aliyun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

type Client interface {
	GetTokenAndExpireTime(ctx context.Context) (string, string, error)
	CheckoutRancherLicense(ctx context.Context) (bool, error)
	GetNumberOfAvailableEntitlements(ctx context.Context) (int, error)
}

type client struct {
	regionID string
}

type Response struct {
	Code       int        `json:"code"`
	RequestID  string     `json:"requestId"`
	InstanceID string     `json:"instanceId"`
	Result     ResultData `json:"result"`
}

type ResultData struct {
	RequestID         string      `json:"RequestId"`
	ServiceInstanceID string      `json:"ServiceInstanceId"`
	LicenseMetadata   LicenseData `json:"LicenseMetadata"`
	TrialType         string      `json:"TrialType"`
	Token             string      `json:"Token"`
	ExpireTime        string      `json:"ExpireTime"`
	ServiceID         string      `json:"ServiceId"`
}

type LicenseData struct {
	TemplateName      string `json:"TemplateName"`
	SpecificationName string `json:"SpecificationName"`
	CustomData        string `json:"CustomData"`
}

func NewClient() (Client, error) {
	c := &client{
		"",
	}
	regionID, err := getRegionID()
	if err != nil {
		return nil, err
	}
	c.regionID = regionID
	return c, nil
}

func (c *client) CheckoutRancherLicense(ctx context.Context) (bool, error) {
	checkOutLicenseResponse, err := c.getRancherLicense(ctx, c.regionID)
	if err != nil {
		fmt.Println("Error checking out license:", err)
		return false, err
	}
	code := checkOutLicenseResponse.Code
	if code != 200 {
		return false, nil
	}

	return true, nil
}

func (c *client) GetNumberOfAvailableEntitlements(ctx context.Context) (int, error) {
	checkOutLicenseResponse, err := c.getRancherLicense(ctx, c.regionID)
	if err != nil {
		fmt.Println("Error checking out license:", err)
		return 0, err
	}
	// test
	checkOutLicenseResponse.Result.LicenseMetadata.CustomData = "15"
	num, err := strconv.Atoi(checkOutLicenseResponse.Result.LicenseMetadata.CustomData)
	if err != nil {
		return 0, err
	}
	return num, nil
}

func (c *client) GetTokenAndExpireTime(ctx context.Context) (string, string, error) {
	checkOutLicenseResponse, err := c.getRancherLicense(ctx, c.regionID)
	if err != nil {
		fmt.Println("Error checking out license:", err)
		return "", "", err
	}

	return checkOutLicenseResponse.Result.Token, checkOutLicenseResponse.Result.ExpireTime, nil
}

func getRegionID() (string, error) {
	resp, err := http.Get("http://100.100.100.200/latest/meta-data/region-id")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (c *client) getRancherLicense(ctx context.Context, regionID string) (*Response, error) {
	url := fmt.Sprintf("https://%s.axt.aliyun.com/computeNest/license/check_out_license", regionID)
	requestData := map[string]string{"Channel": "ComputeNest"}
	requestDataJSON, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	resp, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestDataJSON))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}
