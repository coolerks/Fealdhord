package translation

import (
	"net/http"
)

func buildEngine(cfg resolvedProviderConfig, client *http.Client) (Engine, error) {
	switch cfg.ID {
	case EngineGoogleFree:
		return newGoogleFreeEngine(client), nil
	case EngineGoogleBYOK:
		return newGoogleBYOKEngine(cfg, client)
	case EngineMicrosoftEdge:
		return newMicrosoftEdgeEngine(client), nil
	case EngineMicrosoftAzure:
		return newMicrosoftAzureEngine(cfg, client)
	case EngineAlibaba:
		return newAlibabaEngine(cfg, client)
	case EngineCaiyun:
		return newCaiyunEngine(cfg, client)
	case EngineBaidu:
		return newBaiduEngine(cfg, client)
	case EngineYoudao:
		return newYoudaoEngine(cfg, client)
	default:
		return nil, mtError("ConfigurationError", "不支持的翻译方式")
	}
}
