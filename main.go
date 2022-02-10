package main

import (
	"context"
	"flag"
	"fmt"
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
	ActionAddIP    = "add-ip"
	ActionRemoveIP = "remove-ip"
)

var validActions = []string{ActionAddIP, ActionRemoveIP}

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

	if data == "" {
		logger.Info().Msgf("no data informed")
		return
	}

	awsCfg, ok := getAWSConfig(opts)
	if !ok {
		return
	}

	if opts.Action == ActionAddIP {
		addIP(awsCfg, opts, data)
	} else if action == ActionRemoveIP {
		removeIP(awsCfg, opts, data)
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
		_, _, err := net.ParseCIDR(d)
		if err == nil {
			result = append(result, d)
		}
	}

	return result
}

func addIP(awsCfg aws.Config, opts ExecuteOpts, data string) {
	ipsToAdd := removeInvalidData(data)

	if len(ipsToAdd) == 0 {
		logger.Info().Msg("no IP range to add in the IP Set")
		return
	}

	ipSetAddresses := getIPSetAddresses(awsCfg, opts)

	for _, ip := range ipsToAdd {
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
			len(ipsToAdd), opts.IPSetInfo.ID, opts.IPSetInfo.Name, opts.IPSetInfo.Scope,
		)
	}
}

func removeIP(awsCfg aws.Config, opts ExecuteOpts, data string) {
	ipsToRemove := removeInvalidData(data)

	if len(ipsToRemove) == 0 {
		logger.Info().Msgf("no IP ranges to remove from IP set")
		return
	}

	ipSetAddresses := getIPSetAddresses(awsCfg, opts)
	updatedIPSetAddresses := make([]string, 0)

	for _, ip := range ipSetAddresses {
		if isIPInList(ip, ipsToRemove) {
			continue
		}

		updatedIPSetAddresses = append(updatedIPSetAddresses, ip)
	}

	ok := updateIPSetAddressList(awsCfg, opts, updatedIPSetAddresses)
	if ok {
		logger.Info().Msgf(
			"successfully removed IP ranges (%v) from IP Set [%v] - %v (%v)",
			len(ipsToRemove), opts.IPSetInfo.ID, opts.IPSetInfo.Name, opts.IPSetInfo.Scope,
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
