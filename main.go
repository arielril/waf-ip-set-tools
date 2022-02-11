package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	"github.com/arielril/waf-ip-set-tools/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	"github.com/aws/aws-sdk-go-v2/service/wafv2/types"
)

type AWSOpts struct {
	Profile string
	Region  string
}

type IPSetInfo struct {
	Name  string
	ID    string
	Scope string
}

type ExecuteOpts struct {
	AWSOpts
	IPSetInfo
	Action string
}

var logger = log.GetInstance()

const (
	ActionAddIP       = "add-ip"
	ActionRemoveIP    = "remove-ip"
	ActionClearIPList = "clear"
)

var validActions = []string{ActionAddIP, ActionRemoveIP, ActionClearIPList}

func main() {

	var profile string
	flag.StringVar(&profile, "profile", "", "AWS Profile")

	var region string
	flag.StringVar(&region, "region", "", "AWS Region")

	var action string
	flag.StringVar(&action, "action", "", fmt.Sprintf("Action to execute in the IP Set. Values: %v", validActions))

	var ipsetId string
	flag.StringVar(&ipsetId, "id", "", "IP Set ID")

	var ipsetName string
	flag.StringVar(&ipsetName, "name", "", "IP Set Name")

	var scope string
	flag.StringVar(&scope, "scope", "CLOUDFRONT", "IP Set Scope")

	var data string
	flag.StringVar(&data, "data", "", "IP CIDR list, separated with commas (','). Ex: \"cidr_1,cidr_2,cidr_3\"")

	var fileName string
	flag.StringVar(&fileName, "file", "", "File with a list of IP CIDR")

	flag.Parse()

	opts := ExecuteOpts{
		Action: action,
	}
	opts.AWSOpts = AWSOpts{
		Profile: profile,
		Region:  region,
	}
	opts.IPSetInfo = IPSetInfo{
		Name:  ipsetName,
		ID:    ipsetId,
		Scope: scope,
	}

	if !validExecuteOpts(opts) {
		flag.Usage()
		return
	}

	logger.Info().Msgf("connecting to AWS [%v] [%v] - action (%v)", profile, region, action)

	if action != ActionClearIPList && data == "" && fileName == "" {
		logger.Info().Msgf("no IP list informed")
		return
	}

	awsCfg, ok := getAWSConfig(opts)
	if !ok {
		return
	}

	ipList := make([]string, 0)

	if data != "" {
		ipList = removeInvalidData(data)
	} else if fileName != "" {
		logger.Info().Msgf("loading IP list from file [%v]", fileName)
		data, ok := getIPListFromFile(fileName)
		if !ok {
			return
		}
		ipList = data
	}

	if opts.Action == ActionAddIP {
		addIP(awsCfg, opts, ipList)
	} else if action == ActionRemoveIP {
		removeIP(awsCfg, opts, ipList)
	} else if action == ActionClearIPList {
		clearIPList(awsCfg, opts)
	}
}

func isValidAction(action string) bool {
	for _, ac := range validActions {
		if action == ac {
			return true
		}
	}

	return false
}

func validExecuteOpts(opts ExecuteOpts) bool {
	if opts.AWSOpts.Profile == "" ||
		opts.AWSOpts.Region == "" {
		logger.Print().Msg("invalid ip set aws config")
		return false
	}

	if opts.IPSetInfo.ID == "" ||
		opts.IPSetInfo.Name == "" ||
		opts.IPSetInfo.Scope == "" {
		logger.Print().Msg("invalid ip set information")
		return false
	}

	if !isValidAction(opts.Action) {
		logger.Print().Msg("invalid action")
		return false
	}

	return true
}

func removeInvalidData(data string) []string {
	result := make([]string, 0)

	for _, d := range strings.Split(data, ",") {
		if isValidCIDR(d) {
			result = append(result, d)
		}
	}

	return result
}

func isValidCIDR(ip string) bool {
	_, _, err := net.ParseCIDR(ip)

	if err != nil {
		logger.Warning().Msgf("invalid CIDR [%v]", ip)
		return false
	}

	return true
}

func addIP(awsCfg aws.Config, opts ExecuteOpts, ipList []string) {
	if len(ipList) == 0 {
		logger.Info().Msg("no IP range to add in the IP Set")
		return
	}

	ipSetAddresses := getIPSetAddresses(awsCfg, opts)

	for _, ip := range ipList {
		if !isIPInList(ip, ipSetAddresses) {
			ipSetAddresses = append(ipSetAddresses, ip)
		} else {
			logger.Debug().Msgf("ip range [%v] already in IP set", ip)
		}
	}

	ok := updateIPSetAddressList(awsCfg, opts, ipSetAddresses)
	if ok {
		logger.Info().Msgf(
			"successfully added new IP ranges (%v) in IP Set [%v] - %v (%v)",
			len(ipList), opts.IPSetInfo.ID, opts.IPSetInfo.Name, opts.IPSetInfo.Scope,
		)
	}
}

func removeIP(awsCfg aws.Config, opts ExecuteOpts, ipList []string) {
	if len(ipList) == 0 {
		logger.Info().Msgf("no IP ranges to remove from IP set")
		return
	}

	ipSetAddresses := getIPSetAddresses(awsCfg, opts)
	updatedIPSetAddresses := make([]string, 0)

	for _, ip := range ipSetAddresses {
		if isIPInList(ip, ipList) {
			continue
		}

		updatedIPSetAddresses = append(updatedIPSetAddresses, ip)
	}

	ok := updateIPSetAddressList(awsCfg, opts, updatedIPSetAddresses)
	if ok {
		logger.Info().Msgf(
			"successfully removed IP ranges (%v) from IP Set [%v] - %v (%v)",
			len(ipList), opts.IPSetInfo.ID, opts.IPSetInfo.Name, opts.IPSetInfo.Scope,
		)
	}
}

func clearIPList(awsCfg aws.Config, opts ExecuteOpts) {
	ok := updateIPSetAddressList(awsCfg, opts, make([]string, 0))
	if ok {
		logger.Info().Msgf(
			"cleared IP Set [%v] - %v (%v)",
			opts.IPSetInfo.ID, opts.IPSetInfo.Name, opts.IPSetInfo.Scope,
		)
	}
}

func isIPInList(ip string, lst []string) bool {
	for _, item := range lst {
		if strings.EqualFold(ip, item) {
			return true
		}
	}

	return false
}

func updateIPSetAddressList(awsCfg aws.Config, opts ExecuteOpts, addresses []string) bool {
	client := wafv2.NewFromConfig(awsCfg)

	out, err := client.GetIPSet(
		context.TODO(),
		&wafv2.GetIPSetInput{
			Id:    &opts.IPSetInfo.ID,
			Name:  &opts.IPSetInfo.Name,
			Scope: types.Scope(opts.IPSetInfo.Scope),
		},
	)

	if err != nil {
		logger.Error().Msgf("failed to IP Set LockToken. error: %v", err)
		return false
	}

	_, err = client.UpdateIPSet(
		context.TODO(),
		&wafv2.UpdateIPSetInput{
			Addresses: addresses,
			Id:        &opts.IPSetInfo.ID,
			Name:      &opts.IPSetInfo.Name,
			Scope:     types.Scope(opts.IPSetInfo.Scope),
			LockToken: out.LockToken,
		},
	)

	if err != nil {
		logger.Error().Msgf("failed to update IP Set. error: %v", err)
		return false
	}

	return true
}

func getIPSetAddresses(awsCfg aws.Config, opts ExecuteOpts) []string {
	result := make([]string, 0)

	client := wafv2.NewFromConfig(awsCfg)

	out, err := client.GetIPSet(
		context.TODO(),
		&wafv2.GetIPSetInput{
			Id:    &opts.IPSetInfo.ID,
			Name:  &opts.IPSetInfo.Name,
			Scope: types.Scope(opts.IPSetInfo.Scope),
		},
	)

	if err != nil {
		logger.Error().Msgf("failed to list addresses from ip set. error: %v", err)
		return result
	}

	result = append(result, out.IPSet.Addresses...)
	return result
}

func getAWSConfig(opts ExecuteOpts) (aws.Config, bool) {
	awsCfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(opts.AWSOpts.Region),
		config.WithSharedConfigProfile(opts.AWSOpts.Profile),
	)

	if err != nil {
		logger.Error().Msgf("failed to load config from AWS. error: %v", err)
		return aws.Config{}, false
	}

	return awsCfg, true
}

func getIPListFromFile(fileName string) ([]string, bool) {
	result := make([]string, 0)

	bts, err := ioutil.ReadFile(fileName)
	if err != nil {
		logger.Error().Msgf("failed to read file. error: %v", err)
		return nil, false
	}

	for _, l := range strings.Split(string(bts), "\n") {
		if len(l) == 0 {
			// ignore empty lines
			continue
		}

		cidr := strings.TrimSpace(l)
		if isValidCIDR(cidr) {
			result = append(result, cidr)
		}
	}

	return result, true
}
