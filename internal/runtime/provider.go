package runtime

import "github.com/yusefmosiah/go-choir/internal/provideriface"


func providerPolicyForRuntime(provider provideriface.Provider) provideriface.ProviderPolicy {
	if provider == nil {
		return provideriface.ProviderPolicy{
			ActiveProvider:              "none",
			ModelSelection:              "No provider is configured.",
			SupportsPerRunModelOverride: false,
		}
	}
	if reporter, ok := provider.(interface {
		RuntimeProviderPolicy() provideriface.ProviderPolicy
	}); ok {
		policy := reporter.RuntimeProviderPolicy()
		if policy.ActiveProvider == "" {
			policy.ActiveProvider = provider.ProviderName()
		}
		if policy.ModelSelection == "" {
			policy.ModelSelection = "Provider chooses its default model unless a run explicitly requests a model override."
		}
		return policy
	}
	return provideriface.ProviderPolicy{
		ActiveProvider:              provider.ProviderName(),
		ModelSelection:              "Provider chooses its default model unless a run explicitly requests a model override.",
		SupportsPerRunModelOverride: true,
	}
}
