// Copyright (c) 2017 Tigera, Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package updateprocessors

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	apiv2 "github.com/projectcalico/libcalico-go/lib/apis/v2"
	"github.com/projectcalico/libcalico-go/lib/backend/k8s/conversion"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/selector/parser"
)

func RulesAPIV2ToBackend(ars []apiv2.Rule, ns string) []model.Rule {
	if len(ars) == 0 {
		return nil
	}

	brs := make([]model.Rule, len(ars))
	for idx, ar := range ars {
		brs[idx] = RuleAPIV2ToBackend(ar, ns)
	}
	return brs
}

// RuleAPIToBackend converts an API Rule structure to a Backend Rule structure.
func RuleAPIV2ToBackend(ar apiv2.Rule, ns string) model.Rule {
	var icmpCode, icmpType, notICMPCode, notICMPType *int
	if ar.ICMP != nil {
		icmpCode = ar.ICMP.Code
		icmpType = ar.ICMP.Type
	}

	if ar.NotICMP != nil {
		notICMPCode = ar.NotICMP.Code
		notICMPType = ar.NotICMP.Type
	}

	// Determine which namespaces are impacted by this rule.
	var sourceNSSelector string
	if ar.Source.NamespaceSelector != "" {
		// A namespace selector was given - the rule applies to all namespaces
		// which match this selector.
		sourceNSSelector = parseNamespaceSelector(ar.Source.NamespaceSelector)
	} else if ns != "" {
		// No namespace selector was given and this is a namespaced network policy,
		// so the rule applies only to its own namespace.
		sourceNSSelector = fmt.Sprintf("%s == '%s'", apiv2.LabelNamespace, ns)
	}

	var destNSSelector string
	if ar.Destination.NamespaceSelector != "" {
		// A namespace selector was given - the rule applies to all namespaces
		// which match this selector.
		destNSSelector = parseNamespaceSelector(ar.Destination.NamespaceSelector)
	} else if ns != "" {
		// No namespace selector was given and this is a namespaced network policy,
		// so the rule applies only to its own namespace.
		destNSSelector = fmt.Sprintf("%s == '%s'", apiv2.LabelNamespace, ns)
	}

	// Determine which service account are impacted by this rule.
	var sourceSASelector string
	if ar.Source.ServiceAccounts != nil {
		// A service account selector was given - the rule applies to all serviceaccount
		// which match this selector.
		sourceSASelector = parseServiceAccounts(ar.Source.ServiceAccounts, ns)
	}

	var dstSASelector string
	if ar.Destination.ServiceAccounts != nil {
		// A service account selector was given - the rule applies to all serviceaccount
		// which match this selector.
		dstSASelector = parseServiceAccounts(ar.Destination.ServiceAccounts, ns)
	}

	srcSelector := ar.Source.Selector
	if sourceNSSelector != "" && (ar.Source.Selector != "" || ar.Source.NotSelector != "" || ar.Source.NamespaceSelector != "") {
		// We need to namespace the rule's selector when converting to a v1 object.
		// This occurs when a Selector, NotSelector, or NamespaceSelector is provided and either this is a
		// namespaced NetworkPolicy object, or a NamespaceSelector was defined.
		logCxt := log.WithFields(log.Fields{
			"Namespace":         ns,
			"Selector":          ar.Source.Selector,
			"NamespaceSelector": ar.Source.NamespaceSelector,
			"NotSelector":       ar.Source.NotSelector,
		})
		logCxt.Debug("Update source Selector to include namespace")
		if ar.Source.Selector != "" {
			srcSelector = fmt.Sprintf("(%s) && (%s)", sourceNSSelector, ar.Source.Selector)
		} else {
			srcSelector = sourceNSSelector
		}
	}

	// Append sourceSASelector
	if sourceSASelector != "" && (ar.Source.Selector != "" || ar.Source.NotSelector != "" || ar.Source.ServiceAccounts != nil) {
		logCxt := log.WithFields(log.Fields{
			"Namespace":         ns,
			"Selector":          ar.Source.Selector,
			"NamespaceSelector": ar.Source.NamespaceSelector,
			"ServiceAccountSelector": ar.Source.ServiceAccounts.Names,
			"NotSelector":       ar.Source.NotSelector,
		})
		logCxt.Debug("Update source Selector to include namespace")
		if srcSelector != "" {
			srcSelector = fmt.Sprintf("(%s) && (%s)", sourceSASelector, srcSelector)
		} else {
			srcSelector = sourceSASelector
		}
	}

	dstSelector := ar.Destination.Selector
	if destNSSelector != "" && (ar.Destination.Selector != "" || ar.Destination.NotSelector != "" || ar.Destination.NamespaceSelector != "") {
		// We need to namespace the rule's selector when converting to a v1 object.
		// This occurs when a Selector, NotSelector, or NamespaceSelector is provided and either this is a
		// namespaced NetworkPolicy object, or a NamespaceSelector was defined.
		logCxt := log.WithFields(log.Fields{
			"Namespace":         ns,
			"Selector":          ar.Destination.Selector,
			"NamespaceSelector": ar.Destination.NamespaceSelector,
			"NotSelector":       ar.Destination.NotSelector,
		})
		logCxt.Debug("Update Destination Selector to include namespace")
		if ar.Destination.Selector != "" {
			dstSelector = fmt.Sprintf("(%s) && (%s)", destNSSelector, ar.Destination.Selector)
		} else {
			dstSelector = destNSSelector
		}
	}

	// Append dstSASelector
	if dstSASelector != "" && (ar.Destination.Selector != "" || ar.Destination.NotSelector != "" || ar.Destination.ServiceAccounts != nil) {
		logCxt := log.WithFields(log.Fields{
			"Namespace":         ns,
			"Selector":          ar.Destination.Selector,
			"NamespaceSelector": ar.Destination.NamespaceSelector,
			"ServiceAccountSelector": ar.Destination.ServiceAccounts.Names,
			"NotSelector":       ar.Destination.NotSelector,
		})
		logCxt.Debug("Update Destination Selector to include serviceaccounts")
		if dstSelector != "" {
			dstSelector = fmt.Sprintf("(%s) && (%s)", dstSASelector, dstSelector)
		} else {
			dstSelector = dstSASelector
		}
	}

	return model.Rule{
		Action:      ruleActionAPIV2ToBackend(ar.Action),
		IPVersion:   ar.IPVersion,
		Protocol:    ar.Protocol,
		ICMPCode:    icmpCode,
		ICMPType:    icmpType,
		NotProtocol: ar.NotProtocol,
		NotICMPCode: notICMPCode,
		NotICMPType: notICMPType,

		SrcNets:     convertStringsToNets(ar.Source.Nets),
		SrcSelector: srcSelector,
		SrcPorts:    ar.Source.Ports,
		DstNets:     normalizeIPNets(ar.Destination.Nets),
		DstSelector: dstSelector,
		DstPorts:    ar.Destination.Ports,

		NotSrcNets:     convertStringsToNets(ar.Source.NotNets),
		NotSrcSelector: ar.Source.NotSelector,
		NotSrcPorts:    ar.Source.NotPorts,
		NotDstNets:     normalizeIPNets(ar.Destination.NotNets),
		NotDstSelector: ar.Destination.NotSelector,
		NotDstPorts:    ar.Destination.NotPorts,
	}
}

// parseNamespaceSelector takes a v2 namespace selector and returns the appropriate v1 representation
// by prefixing the keys with the `pcns.` prefix. For example, `k == 'v'` becomes `pcns.k == 'v'`.
func parseNamespaceSelector(s string) string {
	parsedSelector, err := parser.Parse(s)
	if err != nil {
		log.WithError(err).Errorf("Failed to parse namespace selector: %s", s)
		return ""
	}
	parsedSelector.AcceptVisitor(parser.PrefixVisitor{Prefix: conversion.NamespaceLabelPrefix})
	updated := parsedSelector.String()
	log.WithFields(log.Fields{"original": s, "updated": updated}).Debug("Updated namespace selector")
	return updated
}

// parseServiceAccounts takes a v2 service account selector and returns the appropriate v1 representation
// by prefixing the keys with the `pcsa.` prefix. For example, `k == 'v'` becomes `pcsa.k == 'v'`.
func parseServiceAccounts(sam *apiv2.ServiceAccountMatch, ns string) string {
	namespace := ns
	if namespace == "" {
		// not in a namespaced rule; apply to default namespace
		namespace = "default"
	}

	// If the rule itself has a namespace then use it
	if sam.Namespace != "" {
		namespace = sam.Namespace
	}

	parsedSelector, err := parser.Parse(sam.Names)
	if err != nil {
		log.WithError(err).Errorf("Failed to parse service account selector: %s", sam.Names)
		return ""
	}

	prefix := conversion.ServiceAccountLabelPrefix + "." + namespace + "."
	parsedSelector.AcceptVisitor(parser.PrefixVisitor{Prefix: prefix})
	updated := parsedSelector.String()
	log.WithFields(log.Fields{"original": sam.Names, "updated": updated}).Debug("Updated service account selector")
	return updated
}

// normalizeIPNet converts an IPNet to a network by ensuring the IP address is correctly masked.
func normalizeIPNet(n string) *cnet.IPNet {
	if n == "" {
		return nil
	}
	_, ipn, err := cnet.ParseCIDROrIP(n)
	if err != nil {
		return nil
	}
	return ipn.Network()
}

// normalizeIPNets converts an []*IPNet to a slice of networks by ensuring the IP addresses
// are correctly masked.
func normalizeIPNets(nets []string) []*cnet.IPNet {
	if len(nets) == 0 {
		return nil
	}
	out := make([]*cnet.IPNet, len(nets))
	for i, n := range nets {
		out[i] = normalizeIPNet(n)
	}
	return out
}

// ruleActionAPIV2ToBackend converts the rule action field value from the API
// value to the equivalent backend value.
func ruleActionAPIV2ToBackend(action apiv2.Action) string {
	if action == apiv2.Pass {
		return "next-tier"
	}
	return strings.ToLower(string(action))
}

func convertStringsToNets(strs []string) []*cnet.IPNet {
	var nets []*cnet.IPNet
	for _, str := range strs {
		_, ipn, err := cnet.ParseCIDROrIP(str)
		if err != nil {
			continue
		}
		nets = append(nets, ipn)
	}
	return nets
}
