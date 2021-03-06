package main

import "github.com/LunaNode/lobster"
import "github.com/LunaNode/lobster/core/support"
import "github.com/LunaNode/lobster/module/whmcs"

import "github.com/LunaNode/lobster/vmi/lunanode"
import "github.com/LunaNode/lobster/vmi/openstack"
import "github.com/LunaNode/lobster/vmi/solusvm"
import "github.com/LunaNode/lobster/vmi/cloudstack"
import "github.com/LunaNode/lobster/vmi/digitalocean"
import vmfake "github.com/LunaNode/lobster/vmi/fake"
import "github.com/LunaNode/lobster/vmi/linode"
import vmlobster "github.com/LunaNode/lobster/vmi/lobster"
import "github.com/LunaNode/lobster/vmi/vultr"
import "github.com/LunaNode/lobster/vmi/cloug"

import "github.com/LunaNode/lobster/payment/coinbase"
import payfake "github.com/LunaNode/lobster/payment/fake"
import "github.com/LunaNode/lobster/payment/paypal"
import "github.com/LunaNode/lobster/payment/stripe"

import "encoding/json"
import "io/ioutil"
import "log"
import "os"
import "strconv"

type VmConfig struct {
	Name string `json:"name"`

	// one of solusvm, openstack, cloudstack, lobster, lndynamic, fake, digitalocean, vultr, linode, cloug
	Type string `json:"type"`

	// API options (used by solusvm, cloudstack, lobster, lndynamic, digitalocean, vultr, linode)
	ApiId  string `json:"api_id"`
	ApiKey string `json:"api_key"`

	// URL (used by solusvm, lobster, openstack, cloudstack)
	Url string `json:"url"`

	// network ID (used by openstack, cloudstack)
	NetworkId string `json:"network_id"`

	// solusvm options
	VirtType  string `json:"virt_type"`
	NodeGroup string `json:"node_group"`
	Insecure  bool   `json:"insecure"`

	// openstack options
	Username string `json:"username"`
	Password string `json:"password"`
	Tenant   string `json:"tenant"`

	// cloudstack options
	SecretKey string `json:"secret_key"`
	ZoneID    string `json:"zone_id"`

	// region option (used by lobster, lndynamic, digitalocean, vultr, linode)
	Region string `json:"region"`
}

type PaymentConfig struct {
	Name string `json:"name"`

	// one of paypal, coinbase, fake
	Type string `json:"type"`

	// paypal options
	Business        string `json:"business"`
	ReturnUrl       string `json:"return_url"`
	RequireShipping bool   `json:"require_shipping"`

	// coinbase options
	CallbackSecret string `json:"callback_secret"`

	// API options (used by coinbase)
	ApiKey    string `json:"api_key"`
	ApiSecret string `json:"api_secret"`

	// stripe options
	PrivateKey     string `json:"private_key"`
	PublishableKey string `json:"publishable_key"`
}

type JSONConfig struct {
	Vm           []*VmConfig         `json:"vm"`
	Payment      []*PaymentConfig    `json:"payment"`
	Module       []map[string]string `json:"module"`
	SplashRoutes map[string]string   `json:"splash_routes"`
}

type HelperConfig struct {
	Vm []interface{} `json:"vm"`
}

func main() {
	cfgPath := "lobster.cfg"
	if len(os.Args) >= 2 {
		cfgPath = os.Args[1]
	}
	lobster.Setup(cfgPath)

	// associate core
	support.Setup()

	// load json configuration
	jsonPath := cfgPath + ".json"
	jsonConfigBytes, err := ioutil.ReadFile(jsonPath)
	if err != nil {
		log.Fatalf("Error: failed to read json configuration file %s: %s", jsonPath, err.Error())
	}
	var jsonConfig JSONConfig
	err = json.Unmarshal(jsonConfigBytes, &jsonConfig)
	if err != nil {
		log.Fatalf("Error: failed to parse json configuration: %s", err.Error())
	}

	for i, vm := range jsonConfig.Vm {
		log.Printf("Initializing VM interface %s (type=%s)", vm.Name, vm.Type)
		var vmi lobster.VmInterface
		if vm.Type == "openstack" {
			vmi = openstack.MakeOpenStack(vm.Url, vm.Username, vm.Password, vm.Tenant, vm.NetworkId)
		} else if vm.Type == "solusvm" {
			vmi = &solusvm.SolusVM{
				VirtType:  vm.VirtType,
				NodeGroup: vm.NodeGroup,
				Api: &solusvm.API{
					Url:      vm.Url,
					ApiId:    vm.ApiId,
					ApiKey:   vm.ApiKey,
					Insecure: vm.Insecure,
				},
			}
		} else if vm.Type == "lobster" {
			vmi = vmlobster.MakeLobster(vm.Region, vm.Url, vm.ApiId, vm.ApiKey)
		} else if vm.Type == "cloudstack" {
			vmi = cloudstack.MakeCloudStack(vm.Url, vm.ZoneID, vm.NetworkId, vm.ApiKey, vm.SecretKey)
		} else if vm.Type == "lndynamic" {
			vmi = lunanode.MakeLunaNode(vm.Region, vm.ApiId, vm.ApiKey)
		} else if vm.Type == "fake" {
			vmi = new(vmfake.Fake)
		} else if vm.Type == "digitalocean" {
			vmi = digitalocean.MakeDigitalOcean(vm.Region, vm.ApiId)
		} else if vm.Type == "vultr" {
			regionId, err := strconv.Atoi(vm.Region)
			if err != nil {
				log.Fatalf("Error: invalid region ID for vultr interface: %s", vm.Region)
			}
			vmi = vultr.MakeVultr(vm.ApiKey, regionId)
		} else if vm.Type == "linode" {
			datacenterId, err := strconv.Atoi(vm.Region)
			if err != nil {
				log.Fatalf("Error: invalid datacenter ID for linode interface: %s", vm.Region)
			}
			vmi = linode.MakeLinode(vm.ApiKey, datacenterId)
		} else if vm.Type == "cloug" {
			// need to get the JSON data for this interface
			// to do this, we will unmarshal the configuration into HelperConfig, and then
			//   re-marshal the corresponding index
			var helperConfig HelperConfig
			if err := json.Unmarshal(jsonConfigBytes, &helperConfig); err != nil {
				log.Fatalf("Error unmarshaling into helper for cloug interface: %v", err)
			}
			jsonData, err := json.Marshal(helperConfig.Vm[i])
			if err != nil {
				log.Fatalf("Error marshaling from helper for cloug interface: %v", err)
			}
			vmi, err = cloug.MakeCloug(jsonData, vm.Region)
			if err != nil {
				log.Fatalf("Cloug error: %v", err)
			}
		} else {
			log.Fatalf("Encountered unrecognized VM interface type %s", vm.Type)
		}
		log.Println("... initialized successfully")
		lobster.RegisterVmInterface(vm.Name, vmi)
	}

	for _, payment := range jsonConfig.Payment {
		var pi lobster.PaymentInterface
		if payment.Type == "paypal" {
			pi = paypal.MakePaypalPayment(payment.Business, payment.ReturnUrl, payment.RequireShipping)
		} else if payment.Type == "coinbase" {
			pi = coinbase.MakeCoinbasePayment(payment.CallbackSecret, payment.ApiKey, payment.ApiSecret)
		} else if payment.Type == "fake" {
			pi = new(payfake.FakePayment)
		} else if payment.Type == "stripe" {
			pi = stripe.MakeStripePayment(payment.PrivateKey, payment.PublishableKey)
		} else {
			log.Fatalf("Encountered unrecognized payment interface type %s", payment.Type)
		}
		lobster.RegisterPaymentInterface(payment.Name, pi)
	}

	for _, module := range jsonConfig.Module {
		t := module["type"]
		if t == "whmcs" {
			whmcs.MakeWHMCS(module["ip"], module["secret"])
		} else {
			log.Fatalf("Encountered unrecognized module type %s", t)
		}
	}

	for path, template := range jsonConfig.SplashRoutes {
		lobster.RegisterSplashRoute(path, template)
	}

	lobster.Run()
}
