package powervs_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	"github.com/openshift/installer/pkg/asset/installconfig/powervs"
	"github.com/openshift/installer/pkg/asset/installconfig/powervs/mock"
	"github.com/openshift/installer/pkg/ipnet"
	"github.com/openshift/installer/pkg/types"
	powervstypes "github.com/openshift/installer/pkg/types/powervs"
)

type editFunctions []func(ic *types.InstallConfig)

var (
	validRegion                  = "dal"
	validCIDR                    = "192.168.0.0/24"
	validCISInstanceCRN          = "crn:v1:bluemix:public:internet-svcs:global:a/valid-account-id:valid-instance-id::"
	validClusterName             = "valid-cluster-name"
	validDNSZoneID               = "valid-zone-id"
	validBaseDomain              = "valid.base.domain"
	validPowerVSResourceGroup    = "valid-resource-group"
	validPublicSubnetUSSouth1ID  = "public-subnet-us-south-1-id"
	validPublicSubnetUSSouth2ID  = "public-subnet-us-south-2-id"
	validPrivateSubnetUSSouth1ID = "private-subnet-us-south-1-id"
	validPrivateSubnetUSSouth2ID = "private-subnet-us-south-2-id"
	validServiceInstanceID       = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	invalidServiceInstanceID     = "bogus-service-instance-id"
	validSubnets                 = []string{
		validPublicSubnetUSSouth1ID,
		validPublicSubnetUSSouth2ID,
		validPrivateSubnetUSSouth1ID,
		validPrivateSubnetUSSouth2ID,
	}
	validUserID = "valid-user@example.com"
	validZone   = "dal10"

	existingDNSRecordsResponse = []powervs.DNSRecordResponse{
		{
			Name: "valid-dns-record-name-1",
			Type: "valid-dns-record-type",
		},
		{
			Name: "valid-dns-record-name-2",
			Type: "valid-dns-record-type",
		},
	}
	noDNSRecordsResponse   = []powervs.DNSRecordResponse{}
	invalidArchitecture    = func(ic *types.InstallConfig) { ic.ControlPlane.Architecture = "ppc64" }
	cidrInvalid, _         = ipnet.ParseCIDR("192.168.0.0/16")
	invalidMachinePoolCIDR = func(ic *types.InstallConfig) { ic.Networking.MachineNetwork[0].CIDR = *cidrInvalid }
	cidrValid, _           = ipnet.ParseCIDR("192.168.0.0/24")
	validMachinePoolCIDR   = func(ic *types.InstallConfig) { ic.Networking.MachineNetwork[0].CIDR = *cidrValid }
	validVPCRegion         = "us-south"
	invalidVPCRegion       = "foo-bah"
	setValidVPCRegion      = func(ic *types.InstallConfig) { ic.Platform.PowerVS.VPCRegion = validVPCRegion }
	validRG                = "valid-resource-group"
	anotherValidRG         = "another-valid-resource-group"
	validVPCID             = "valid-id"
	anotherValidVPCID      = "another-valid-id"
	validVPC               = "valid-vpc"
	setValidVPCName        = func(ic *types.InstallConfig) { ic.Platform.PowerVS.VPCName = validVPC }
	anotherValidVPC        = "another-valid-vpc"
	invalidVPC             = "bogus-vpc"
	validVPCs              = []vpcv1.VPC{
		{
			Name: &validVPC,
			ID:   &validVPCID,
			ResourceGroup: &vpcv1.ResourceGroupReference{
				Name: &validRG,
				ID:   &validRG,
			},
		},
		{
			Name: &anotherValidVPC,
			ID:   &anotherValidVPCID,
			ResourceGroup: &vpcv1.ResourceGroupReference{
				Name: &anotherValidRG,
				ID:   &anotherValidRG,
			},
		},
	}
	validVPCSubnet   = "valid-vpc-subnet"
	invalidVPCSubnet = "invalid-vpc-subnet"
	wrongVPCSubnet   = "wrong-vpc-subnet"
	validSubnet      = &vpcv1.Subnet{
		Name: &validRG,
		VPC: &vpcv1.VPCReference{
			Name: &validVPC,
			ID:   &validVPCID,
		},
		ResourceGroup: &vpcv1.ResourceGroupReference{
			Name: &validRG,
			ID:   &validRG,
		},
	}
	wrongSubnet = &vpcv1.Subnet{
		Name: &validRG,
		VPC: &vpcv1.VPCReference{
			Name: &anotherValidVPC,
			ID:   &anotherValidVPCID,
		},
		ResourceGroup: &vpcv1.ResourceGroupReference{
			Name: &validRG,
			ID:   &validRG,
		},
	}
	regionWithPER    = "dal10"
	regionWithoutPER = "foo99"
	regionPERUnknown = "bah77"
	mapWithPERFalse  = map[string]bool{
		"disaster-recover-site": true,
		"power-edge-router":     false,
		"vpn-connections":       true,
	}
	mapWithPERTrue = map[string]bool{
		"disaster-recover-site": true,
		"power-edge-router":     true,
		"vpn-connections":       true,
	}
	mapPERUnknown = map[string]bool{
		"disaster-recover-site": true,
		"power-vpn-connections": false,
	}
)

func validInstallConfig() *types.InstallConfig {
	return &types.InstallConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: validClusterName,
		},
		BaseDomain: validBaseDomain,
		Networking: &types.Networking{
			MachineNetwork: []types.MachineNetworkEntry{
				{CIDR: *ipnet.MustParseCIDR(validCIDR)},
			},
		},
		Publish: types.ExternalPublishingStrategy,
		Platform: types.Platform{
			PowerVS: validMinimalPlatform(),
		},
		ControlPlane: &types.MachinePool{
			Architecture: "ppc64le",
		},
		Compute: []types.MachinePool{{
			Architecture: "ppc64le",
		}},
	}
}

func validMinimalPlatform() *powervstypes.Platform {
	return &powervstypes.Platform{
		PowerVSResourceGroup: validPowerVSResourceGroup,
		Region:               validRegion,
		ServiceInstanceID:    validServiceInstanceID,
		UserID:               validUserID,
		Zone:                 validZone,
	}
}

func validMachinePool() *powervstypes.MachinePool {
	return &powervstypes.MachinePool{}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name     string
		edits    editFunctions
		errorMsg string
	}{
		{
			name:     "valid install config",
			edits:    editFunctions{},
			errorMsg: "",
		},
		{
			name:     "invalid architecture",
			edits:    editFunctions{invalidArchitecture},
			errorMsg: `^controlPlane.architecture\: Unsupported value\: \"ppc64\"\: supported values: \"ppc64le\"`,
		},
		{
			name:     "invalid machine pool CIDR",
			edits:    editFunctions{invalidMachinePoolCIDR},
			errorMsg: `Networking.MachineNetwork.CIDR: Invalid value: "192.168.0.0/16": Machine Pool CIDR must be /24.`,
		},
		{
			name:     "valid machine pool CIDR",
			edits:    editFunctions{validMachinePoolCIDR},
			errorMsg: "",
		},
	}

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			editedInstallConfig := validInstallConfig()
			for _, edit := range tc.edits {
				edit(editedInstallConfig)
			}

			aggregatedErrors := powervs.Validate(editedInstallConfig)
			if tc.errorMsg != "" {
				assert.Regexp(t, tc.errorMsg, aggregatedErrors)
			} else {
				assert.NoError(t, aggregatedErrors)
			}
		})
	}
}

func TestValidatePreExistingPublicDNS(t *testing.T) {
	cases := []struct {
		name     string
		edits    editFunctions
		errorMsg string
	}{
		{
			name:     "no pre-existing DNS records",
			errorMsg: "",
		},
		{
			name:     "pre-existing DNS records",
			errorMsg: `^\[baseDomain\: Duplicate value\: \"record api\.valid-cluster-name\.valid\.base\.domain already exists in CIS zone \(valid-zone-id\) and might be in use by another cluster, please remove it to continue\", baseDomain\: Duplicate value\: \"record api-int\.valid-cluster-name\.valid\.base\.domain already exists in CIS zone \(valid-zone-id\) and might be in use by another cluster, please remove it to continue\"\]$`,
		},
		{
			name:     "cannot get zone ID",
			errorMsg: `^baseDomain: Internal error$`,
		},
		{
			name:     "cannot get DNS records",
			errorMsg: `^baseDomain: Internal error$`,
		},
	}
	setMockEnvVars()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	powervsClient := mock.NewMockAPI(mockCtrl)
	metadata := mock.NewMockMetadataAPI(mockCtrl)

	dnsRecordNames := [...]string{fmt.Sprintf("api.%s.%s", validClusterName, validBaseDomain), fmt.Sprintf("api-int.%s.%s", validClusterName, validBaseDomain)}

	// Mock common to all tests
	metadata.EXPECT().CISInstanceCRN(gomock.Any()).Return(validCISInstanceCRN, nil).AnyTimes()

	// Mocks: no pre-existing DNS records
	powervsClient.EXPECT().GetDNSZoneIDByName(gomock.Any(), validBaseDomain, types.ExternalPublishingStrategy).Return(validDNSZoneID, nil)
	for _, dnsRecordName := range dnsRecordNames {
		powervsClient.EXPECT().GetDNSRecordsByName(gomock.Any(), validCISInstanceCRN, validDNSZoneID, dnsRecordName, types.ExternalPublishingStrategy).Return(noDNSRecordsResponse, nil)
	}

	// Mocks: pre-existing DNS records
	powervsClient.EXPECT().GetDNSZoneIDByName(gomock.Any(), validBaseDomain, types.ExternalPublishingStrategy).Return(validDNSZoneID, nil)
	for _, dnsRecordName := range dnsRecordNames {
		powervsClient.EXPECT().GetDNSRecordsByName(gomock.Any(), validCISInstanceCRN, validDNSZoneID, dnsRecordName, types.ExternalPublishingStrategy).Return(existingDNSRecordsResponse, nil)
	}

	// Mocks: cannot get zone ID
	powervsClient.EXPECT().GetDNSZoneIDByName(gomock.Any(), validBaseDomain, types.ExternalPublishingStrategy).Return("", fmt.Errorf(""))

	// Mocks: cannot get DNS records
	powervsClient.EXPECT().GetDNSZoneIDByName(gomock.Any(), validBaseDomain, types.ExternalPublishingStrategy).Return(validDNSZoneID, nil)
	for _, dnsRecordName := range dnsRecordNames {
		powervsClient.EXPECT().GetDNSRecordsByName(gomock.Any(), validCISInstanceCRN, validDNSZoneID, dnsRecordName, types.ExternalPublishingStrategy).Return(nil, fmt.Errorf(""))
	}

	// Run tests
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aggregatedErrors := powervs.ValidatePreExistingDNS(powervsClient, validInstallConfig(), metadata)
			if tc.errorMsg != "" {
				assert.Regexp(t, tc.errorMsg, aggregatedErrors)
			} else {
				assert.NoError(t, aggregatedErrors)
			}
		})
	}
}

func TestValidateCustomVPCSettings(t *testing.T) {
	cases := []struct {
		name     string
		edits    editFunctions
		errorMsg string
	}{
		{
			name: "invalid VPC region supplied alone",
			edits: editFunctions{
				func(ic *types.InstallConfig) {
					ic.Platform.PowerVS.VPCRegion = invalidVPCRegion
				},
			},
			errorMsg: fmt.Sprintf(`VPC.vpcRegion: Not found: "%s"`, invalidVPCRegion),
		},
		{
			name: "valid VPC region supplied alone",
			edits: editFunctions{
				func(ic *types.InstallConfig) {
					ic.Platform.PowerVS.VPCRegion = validVPCRegion
				},
			},
			errorMsg: "",
		},
		{
			name: "invalid VPC name supplied, without VPC region, not found near PowerVS region",
			edits: editFunctions{
				func(ic *types.InstallConfig) {
					ic.Platform.PowerVS.VPCName = invalidVPC
				},
			},
			errorMsg: fmt.Sprintf(`VPC.vpcName: Not found: "%s"`, invalidVPC),
		},
		{
			name: "valid VPC name supplied, without VPC region, but found close to PowerVS region",
			edits: editFunctions{
				setValidVPCName,
			},
			errorMsg: "",
		},
		{
			name: "valid VPC name, with invalid VPC region",
			edits: editFunctions{
				setValidVPCName,
				func(ic *types.InstallConfig) {
					ic.Platform.PowerVS.VPCRegion = invalidVPCRegion
				},
			},
			errorMsg: "VPC.vpcRegion: Internal error: unknown region",
		},
		{
			name: "valid VPC name, valid VPC region",
			edits: editFunctions{
				setValidVPCName,
				setValidVPCRegion,
			},
			errorMsg: "",
		},
		{
			name: "VPC subnet supplied, without vpcName",
			edits: editFunctions{
				func(ic *types.InstallConfig) {
					ic.Platform.PowerVS.VPCSubnets = []string{validVPCSubnet}
				},
			},
			errorMsg: `VPC.vpcSubnets: Invalid value: "null": invalid without vpcName`,
		},
		{
			name: "VPC found, but not subnet",
			edits: editFunctions{
				setValidVPCName,
				func(ic *types.InstallConfig) {
					ic.Platform.PowerVS.VPCSubnets = []string{invalidVPCSubnet}
				},
			},
			errorMsg: "VPC.vpcSubnets: Internal error",
		},
		{
			name: "VPC found, subnet found as well, but not attached to the VPC",
			edits: editFunctions{
				setValidVPCName,
				func(ic *types.InstallConfig) {
					ic.Platform.PowerVS.VPCSubnets = []string{wrongVPCSubnet}
				},
			},
			errorMsg: `VPC.vpcSubnets: Invalid value: "null": not attached to VPC`,
		},
		{
			name: "region specified, VPC found, subnet found, and properly attached",
			edits: editFunctions{
				setValidVPCName,
				setValidVPCRegion,
				func(ic *types.InstallConfig) {
					ic.Platform.PowerVS.VPCSubnets = []string{validVPCSubnet}
				},
			},
			errorMsg: "",
		},
	}
	setMockEnvVars()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	powervsClient := mock.NewMockAPI(mockCtrl)

	// Mocks: invalid VPC region only
	// nothing to mock

	// Mocks: valid VPC region only
	// nothing to mock

	// Mocks: invalid VPC name results in error
	powervsClient.EXPECT().GetVPCs(gomock.Any(), validVPCRegion).Return(validVPCs, nil)

	// Mocks: valid VPC name only, no issues
	powervsClient.EXPECT().GetVPCs(gomock.Any(), validVPCRegion).Return(validVPCs, nil)

	// Mocks: valid VPC name, invalid VPC region
	powervsClient.EXPECT().GetVPCs(gomock.Any(), invalidVPCRegion).Return(nil, fmt.Errorf("unknown region"))

	// Mocks: valid VPC name, valid VPC region, all good
	powervsClient.EXPECT().GetVPCs(gomock.Any(), validVPCRegion).Return(validVPCs, nil)

	// Mocks: subnet specified, without vpcName, invalid
	// nothing to mock

	// Mocks: valid VPC name, but Subnet not found
	powervsClient.EXPECT().GetVPCs(gomock.Any(), validVPCRegion).Return(validVPCs, nil)
	powervsClient.EXPECT().GetSubnetByName(gomock.Any(), invalidVPCSubnet, validVPCRegion).Return(nil, fmt.Errorf(""))

	// Mocks: valid VPC name, but wrong Subnet (present, but not attached)
	powervsClient.EXPECT().GetVPCs(gomock.Any(), validVPCRegion).Return(validVPCs, nil)
	powervsClient.EXPECT().GetSubnetByName(gomock.Any(), wrongVPCSubnet, validVPCRegion).Return(wrongSubnet, nil)

	// Mocks: region specified, valid VPC, valid region, valid Subnet, all good
	powervsClient.EXPECT().GetVPCs(gomock.Any(), validVPCRegion).Return(validVPCs, nil)
	powervsClient.EXPECT().GetSubnetByName(gomock.Any(), validVPCSubnet, validVPCRegion).Return(validSubnet, nil)

	// Run tests
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			editedInstallConfig := validInstallConfig()
			for _, edit := range tc.edits {
				edit(editedInstallConfig)
			}

			aggregatedErrors := powervs.ValidateCustomVPCSetup(powervsClient, editedInstallConfig)
			if tc.errorMsg != "" {
				assert.Regexp(t, tc.errorMsg, aggregatedErrors)
			} else {
				assert.NoError(t, aggregatedErrors)
			}
		})
	}
}

func createControlPlanes(numControlPlanes int, controlPlane *machinev1.PowerVSMachineProviderConfig) []machinev1beta1.Machine {
	controlPlanes := make([]machinev1beta1.Machine, numControlPlanes)

	for i := range controlPlanes {
		masterName := fmt.Sprintf("rdr-hamzy-test3-syd04-zwmgs-master-%d", i)
		controlPlanes[i].TypeMeta = metav1.TypeMeta{
			Kind:       "Machine",
			APIVersion: "machine.openshift.io/v1beta1",
		}
		controlPlanes[i].ObjectMeta = metav1.ObjectMeta{
			Name:      masterName,
			Namespace: "openshift-machine-api",
			Labels:    make(map[string]string),
		}
		controlPlanes[i].Labels["machine.openshift.io/cluster-api-cluster"] = "rdr-hamzy-test3-syd04-zwmgs"
		controlPlanes[i].Labels["machine.openshift.io/cluster-api-machine-role"] = "master"
		controlPlanes[i].Labels["machine.openshift.io/cluster-api-machine-type"] = "master"

		controlPlanes[i].Spec.ProviderSpec = machinev1beta1.ProviderSpec{
			Value: &runtime.RawExtension{
				Raw:    nil,
				Object: controlPlane,
			},
		}
	}

	return controlPlanes
}

func createComputes(numComputes int32, compute *machinev1.PowerVSMachineProviderConfig) []machinev1beta1.MachineSet {
	computes := make([]machinev1beta1.MachineSet, 1)

	computes[0].Spec.Replicas = &numComputes

	computes[0].Spec.Template.Spec.ProviderSpec = machinev1beta1.ProviderSpec{
		Value: &runtime.RawExtension{
			Raw:    nil,
			Object: compute,
		},
	}

	return computes
}

func TestSystemPool(t *testing.T) {
	setMockEnvVars()

	dedicatedControlPlane := machinev1.PowerVSMachineProviderConfig{
		TypeMeta:      metav1.TypeMeta{Kind: "PowerVSMachineProviderConfig", APIVersion: "machine.openshift.io/v1"},
		KeyPairName:   "rdr-hamzy-test3-syd04-vcwtz-key",
		SystemType:    "e980",
		ProcessorType: "Dedicated",
		Processors:    intstr.IntOrString{Type: intstr.Int, IntVal: 1},
		MemoryGiB:     32,
	}

	dedicatedControlPlanes := createControlPlanes(5, &dedicatedControlPlane)

	dedicatedCompute := machinev1.PowerVSMachineProviderConfig{
		TypeMeta:      metav1.TypeMeta{Kind: "PowerVSMachineProviderConfig", APIVersion: "machine.openshift.io/v1"},
		KeyPairName:   "rdr-hamzy-test3-syd04-vcwtz-key",
		SystemType:    "e980",
		ProcessorType: "Dedicated",
		Processors:    intstr.IntOrString{Type: intstr.Int, IntVal: 1},
		MemoryGiB:     32,
	}

	dedicatedComputes := createComputes(3, &dedicatedCompute)

	systemPoolNEComputeCores := &models.System{
		Cores:  func(f float64) *float64 { return &f }(2),
		ID:     1,
		Memory: func(i int64) *int64 { return &i }(256),
	}
	systemPoolsNEComputeCores := models.SystemPools{
		"NotEnoughComputeCores": models.SystemPool{
			Capacity:           systemPoolNEComputeCores,
			CoreMemoryRatio:    float64(1.0),
			MaxAvailable:       systemPoolNEComputeCores,
			MaxCoresAvailable:  systemPoolNEComputeCores,
			MaxMemoryAvailable: systemPoolNEComputeCores,
			SharedCoreRatio: &models.MinMaxDefault{
				Default: func(f float64) *float64 { return &f }(4),
				Max:     func(f float64) *float64 { return &f }(4),
				Min:     func(f float64) *float64 { return &f }(1),
			},
			Systems: []*models.System{
				systemPoolNEComputeCores,
			},
			Type: "e980",
		},
	}
	systemPoolNEWorkerCores := &models.System{
		Cores:  func(f float64) *float64 { return &f }(6),
		ID:     1,
		Memory: func(i int64) *int64 { return &i }(256),
	}
	systemPoolsNEWorkerCores := models.SystemPools{
		"NotEnoughWorkerCores": models.SystemPool{
			Capacity:           systemPoolNEWorkerCores,
			CoreMemoryRatio:    float64(1.0),
			MaxAvailable:       systemPoolNEWorkerCores,
			MaxCoresAvailable:  systemPoolNEWorkerCores,
			MaxMemoryAvailable: systemPoolNEWorkerCores,
			SharedCoreRatio: &models.MinMaxDefault{
				Default: func(f float64) *float64 { return &f }(4),
				Max:     func(f float64) *float64 { return &f }(4),
				Min:     func(f float64) *float64 { return &f }(1),
			},
			Systems: []*models.System{
				systemPoolNEWorkerCores,
			},
			Type: "e980",
		},
	}
	systemPoolNEComputeMemory := &models.System{
		Cores:  func(f float64) *float64 { return &f }(8),
		ID:     1,
		Memory: func(i int64) *int64 { return &i }(32),
	}
	systemPoolsNEComputeMemory := models.SystemPools{
		"NotEnoughComputeMemory": models.SystemPool{
			Capacity:           systemPoolNEComputeMemory,
			CoreMemoryRatio:    float64(1.0),
			MaxAvailable:       systemPoolNEComputeMemory,
			MaxCoresAvailable:  systemPoolNEComputeMemory,
			MaxMemoryAvailable: systemPoolNEComputeMemory,
			SharedCoreRatio: &models.MinMaxDefault{
				Default: func(f float64) *float64 { return &f }(4),
				Max:     func(f float64) *float64 { return &f }(4),
				Min:     func(f float64) *float64 { return &f }(1),
			},
			Systems: []*models.System{
				systemPoolNEComputeMemory,
			},
			Type: "e980",
		},
	}
	systemPoolNEWorkerMemory := &models.System{
		Cores:  func(f float64) *float64 { return &f }(8),
		ID:     1,
		Memory: func(i int64) *int64 { return &i }(192),
	}
	systemPoolsNEWorkerMemory := models.SystemPools{
		"NotEnoughWorkerMemory": models.SystemPool{
			Capacity:           systemPoolNEWorkerMemory,
			CoreMemoryRatio:    float64(1.0),
			MaxAvailable:       systemPoolNEWorkerMemory,
			MaxCoresAvailable:  systemPoolNEWorkerMemory,
			MaxMemoryAvailable: systemPoolNEWorkerMemory,
			SharedCoreRatio: &models.MinMaxDefault{
				Default: func(f float64) *float64 { return &f }(4),
				Max:     func(f float64) *float64 { return &f }(4),
				Min:     func(f float64) *float64 { return &f }(1),
			},
			Systems: []*models.System{
				systemPoolNEWorkerMemory,
			},
			Type: "e980",
		},
	}
	systemPoolGood := &models.System{
		Cores:  func(f float64) *float64 { return &f }(8),
		ID:     1,
		Memory: func(i int64) *int64 { return &i }(256),
	}
	systemPoolsGood := models.SystemPools{
		"Enough": models.SystemPool{
			Capacity:           systemPoolGood,
			CoreMemoryRatio:    float64(1.0),
			MaxAvailable:       systemPoolGood,
			MaxCoresAvailable:  systemPoolGood,
			MaxMemoryAvailable: systemPoolGood,
			SharedCoreRatio: &models.MinMaxDefault{
				Default: func(f float64) *float64 { return &f }(4),
				Max:     func(f float64) *float64 { return &f }(4),
				Min:     func(f float64) *float64 { return &f }(1),
			},
			Systems: []*models.System{
				systemPoolGood,
			},
			Type: "e980",
		},
	}

	err := powervs.ValidateCapacityWithPools(dedicatedControlPlanes, dedicatedComputes, systemPoolsNEComputeCores)
	assert.EqualError(t, err, "Not enough cores available (2) for the compute nodes (need 5)")

	err = powervs.ValidateCapacityWithPools(dedicatedControlPlanes, dedicatedComputes, systemPoolsNEWorkerCores)
	assert.EqualError(t, err, "Not enough cores available (1) for the worker nodes (need 3)")

	err = powervs.ValidateCapacityWithPools(dedicatedControlPlanes, dedicatedComputes, systemPoolsNEComputeMemory)
	assert.EqualError(t, err, "Not enough memory available (32) for the compute nodes (need 160)")

	err = powervs.ValidateCapacityWithPools(dedicatedControlPlanes, dedicatedComputes, systemPoolsNEWorkerMemory)
	assert.EqualError(t, err, "Not enough memory available (32) for the worker nodes (need 96)")

	err = powervs.ValidateCapacityWithPools(dedicatedControlPlanes, dedicatedComputes, systemPoolsGood)
	assert.Empty(t, err)
}

func TestValidatePERAvailability(t *testing.T) {
	cases := []struct {
		name     string
		edits    editFunctions
		errorMsg string
	}{
		{
			name: "Region without PER",
			edits: editFunctions{
				func(ic *types.InstallConfig) {
					ic.Platform.PowerVS.Zone = regionWithoutPER
				},
			},
			errorMsg: fmt.Sprintf("power-edge-router is not available at: %s", regionWithoutPER),
		},
		{
			name: "Region with PER",
			edits: editFunctions{
				func(ic *types.InstallConfig) {
					ic.Platform.PowerVS.Zone = regionWithPER
					ic.Platform.PowerVS.ServiceInstanceID = validServiceInstanceID
				},
			},
			errorMsg: "",
		},
		{
			name: "Region with no PER availability info",
			edits: editFunctions{
				func(ic *types.InstallConfig) {
					ic.Platform.PowerVS.Zone = regionPERUnknown
				},
			},
			errorMsg: fmt.Sprintf("power-edge-router capability unknown at: %s", regionPERUnknown),
		},
		{
			name: "Region with PER, but with invalid Workspace ID",
			edits: editFunctions{
				func(ic *types.InstallConfig) {
					ic.Platform.PowerVS.Zone = regionWithPER
					ic.Platform.PowerVS.ServiceInstanceID = invalidServiceInstanceID
				},
			},
			errorMsg: fmt.Sprintf("power-edge-router is not available in workspace: %s", invalidServiceInstanceID),
		},
	}
	setMockEnvVars()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	powervsClient := mock.NewMockAPI(mockCtrl)

	// Mocks: PER-absent region results in false
	powervsClient.EXPECT().GetDatacenterCapabilities(gomock.Any(), regionWithoutPER).Return(mapWithPERFalse, nil)

	// Mocks: PER-enabled region results in true
	powervsClient.EXPECT().GetDatacenterCapabilities(gomock.Any(), regionWithPER).Return(mapWithPERTrue, nil)
	powervsClient.EXPECT().GetWorkspaceCapabilities(gomock.Any(), validServiceInstanceID).Return(mapWithPERTrue, nil)

	// Mocks: PER-unknown region results in false
	powervsClient.EXPECT().GetDatacenterCapabilities(gomock.Any(), regionPERUnknown).Return(mapPERUnknown, nil)

	// Mocks: PER-enabled region, but bogus Service Instance results in false
	powervsClient.EXPECT().GetDatacenterCapabilities(gomock.Any(), regionWithPER).Return(mapWithPERTrue, nil)
	powervsClient.EXPECT().GetWorkspaceCapabilities(gomock.Any(), invalidServiceInstanceID).Return(mapWithPERFalse, nil)

	// Run tests
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			editedInstallConfig := validInstallConfig()
			for _, edit := range tc.edits {
				edit(editedInstallConfig)
			}

			aggregatedErrors := powervs.ValidatePERAvailability(powervsClient, editedInstallConfig)
			if tc.errorMsg != "" {
				assert.Regexp(t, tc.errorMsg, aggregatedErrors)
			} else {
				assert.NoError(t, aggregatedErrors)
			}
		})
	}
}

func setMockEnvVars() {
	os.Setenv("POWERVS_AUTH_FILEPATH", "./tmp/powervs/config.json")
	os.Setenv("IBMID", "foo")
	os.Setenv("IC_API_KEY", "foo")
	os.Setenv("IBMCLOUD_REGION", "foo")
	os.Setenv("IBMCLOUD_ZONE", "foo")
}
