package controller

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	lObjects "github.com/vngcloud/vngcloud-go-sdk/vngcloud/objects"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/pool"
	"github.com/vngcloud/vngcloud-ingress-controller/pkg/consts"
	vErrors "github.com/vngcloud/vngcloud-ingress-controller/pkg/utils/errors"
	apiv1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	corelisters "k8s.io/client-go/listers/core/v1"
	nwlisters "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// hash a string to a string have 10 char
func HashString(str string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(str)))
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func TrimString(str string, length int) string {
	return str[:MinInt(len(str), length)]
}

// NodeNames get all the node names.
func NodeNames(nodes []*apiv1.Node) []string {
	ret := make([]string, len(nodes))
	for i, node := range nodes {
		ret[i] = node.Name
	}
	return ret
}

// NodeSlicesEqual check if two nodes equals to each other.
func NodeSlicesEqual(x, y []*apiv1.Node) bool {
	if len(x) != len(y) {
		return false
	}
	return stringSlicesEqual(NodeNames(x), NodeNames(y))
}

func stringSlicesEqual(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}
	if !sort.StringsAreSorted(x) {
		sort.Strings(x)
	}
	if !sort.StringsAreSorted(y) {
		sort.Strings(y)
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func validateName(newName string) string {
	for _, char := range newName {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '-' && char != '.' {
			newName = strings.ReplaceAll(newName, string(char), "-")
		}
	}
	return TrimString(newName, consts.DEFAULT_PORTAL_NAME_LENGTH)
}

func CheckIfPoolMemberExist(mems []*pool.Member, mem *pool.Member) bool {
	for _, r := range mems {
		if r.IpAddress == mem.IpAddress &&
			r.Port == mem.Port &&
			r.MonitorPort == mem.MonitorPort &&
			r.Backup == mem.Backup &&
			// r.Name == mem.Name &&
			r.Weight == mem.Weight {
			return true
		}
	}
	return false
}

func ConvertObjectToPoolMember(obj *lObjects.Member) *pool.Member {
	return &pool.Member{
		IpAddress:   obj.Address,
		Port:        obj.ProtocolPort,
		MonitorPort: obj.MonitorPort,
		Backup:      obj.Backup,
		Weight:      obj.Weight,
		Name:        obj.Name,
	}
}

func ConvertObjectToPoolMemberArray(obj []*lObjects.Member) []*pool.Member {
	ret := make([]*pool.Member, len(obj))
	for i, m := range obj {
		ret[i] = ConvertObjectToPoolMember(m)
	}
	return ret
}

func ComparePoolMembers(p1, p2 []*pool.Member) bool {
	if len(p1) != len(p2) {
		return false
	}
	for _, m := range p2 {
		if !CheckIfPoolMemberExist(p1, m) {
			klog.Infof("member in pool not exist: %v", m)
			return false
		}
	}
	return true
}

const (
	// Define the regular expression pattern
	patternPrefix = `vngcloud:\/\/`
	rawPrefix     = `vngcloud://`
	pattern       = "^" + patternPrefix + "ins-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"
)

var (
	vngCloudProviderIDRegex = regexp.MustCompile(pattern)
)

func GetListProviderID(pnodes []*apiv1.Node) []string {
	var providerIDs []string
	for _, node := range pnodes {
		if node != nil && (matchCloudProviderPattern(node.Spec.ProviderID)) {
			providerIDs = append(providerIDs, getProviderID(node))
		}
	}

	return providerIDs
}

func RandStr(l int) string {
	buff := make([]byte, int(math.Ceil(float64(l)/2)))
	_, _ = rand.Read(buff)
	str := hex.EncodeToString(buff)
	return str[:l]
}

func matchCloudProviderPattern(pproviderID string) bool {
	return vngCloudProviderIDRegex.MatchString(pproviderID)
}

func getProviderID(pnode *apiv1.Node) string {
	return pnode.Spec.ProviderID[len(rawPrefix):len(pnode.Spec.ProviderID)]
}

func getNodeMembersAddr(nodeObjs []*apiv1.Node) []string {
	var nodeAddr []string
	for _, node := range nodeObjs {
		addr, err := getNodeAddressForLB(node)
		if err != nil {
			// Node failure, do not create member
			klog.Warningf("failed to get node %s address: %v", node.Name, err)
			continue
		}
		nodeAddr = append(nodeAddr, addr)
	}
	return nodeAddr
}

func getIngress(ingressLister nwlisters.IngressLister, key string) (*nwv1.Ingress, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	ingress, err := ingressLister.Ingresses(namespace).Get(name)
	if err != nil {
		return nil, err
	}
	return ingress, nil
}

func getService(serviceLister corelisters.ServiceLister, key string) (*apiv1.Service, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	service, err := serviceLister.Services(namespace).Get(name)
	if err != nil {
		return nil, err
	}

	return service, nil
}

func getServiceNodePort(serviceLister corelisters.ServiceLister, name string, serviceBackend *nwv1.IngressServiceBackend) (int, error) {
	var portInfo intstr.IntOrString
	if serviceBackend.Port.Name != "" {
		portInfo.Type = intstr.String
		portInfo.StrVal = serviceBackend.Port.Name
	} else {
		portInfo.Type = intstr.Int
		portInfo.IntVal = serviceBackend.Port.Number
	}

	svc, err := getService(serviceLister, name)
	if err != nil {
		return 0, err
	}

	var nodePort int
	ports := svc.Spec.Ports
	for _, p := range ports {
		if portInfo.Type == intstr.Int && int(p.Port) == portInfo.IntValue() {
			nodePort = int(p.NodePort)
			break
		}
		if portInfo.Type == intstr.String && p.Name == portInfo.StrVal {
			nodePort = int(p.NodePort)
			break
		}
	}

	if nodePort == 0 {
		return 0, fmt.Errorf("failed to find nodeport for service %s:%s", name, portInfo.String())
	}

	return nodePort, nil
}

func ensureNodesInCluster(pserver []*lObjects.Server) (string, error) {
	subnetMapping := make(map[string]int)
	for _, server := range pserver {
		subnets := listSubnetIDs(server)
		for _, subnet := range subnets {
			if smi, ok := subnetMapping[subnet]; !ok {
				subnetMapping[subnet] = 1
			} else {
				subnetMapping[subnet] = smi + 1
			}
		}
	}

	for subnet, count := range subnetMapping {
		if count == len(pserver) && len(subnet) > 0 {
			return subnet, nil
		}
	}

	return "", vErrors.ErrNodesAreNotSameSubnet
}

func listSubnetIDs(s *lObjects.Server) []string {
	var subnets []string
	if s == nil {
		return subnets
	}

	for _, subnet := range s.InternalInterfaces {
		subnets = append(subnets, subnet.SubnetUuid)
	}

	return subnets
}
