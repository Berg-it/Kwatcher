package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	corev1beta1 "kwatch.cloudcorner.org/k-watcher/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Helper method to call external webservice
func (r *KwatcherReconciler) callWebService(ctx context.Context, prov corev1beta1.KwatcherProvider, conf corev1beta1.KwatcherConfig, apiKeyType string, apiSecret string) (map[string]interface{}, error) {
	client := &http.Client{}

	req, err := http.NewRequestWithContext(ctx, "GET", prov.Url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add Authorization header if secret is provided
	if conf.Secret != "" {
		req.Header.Add(strings.TrimSpace(apiKeyType), strings.TrimSpace(apiSecret))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return result, nil
}

// Helper method to convert map to json
func convertToJson(jsonResponse map[string]interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(jsonResponse)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

// CompareResult struct
type KcompareResult struct {
	Result     ctrl.Result
	Err        error
	needUpdate bool
}

// Compare the new json data with the current configmap
func compareAndUpdateConfig(newJsonData []byte, currentConfig *corev1.ConfigMap) KcompareResult {

	var existingConfigMap map[string]interface{}
	if err := json.Unmarshal([]byte(currentConfig.Data["config"]), &existingConfigMap); err != nil {
		return KcompareResult{Result: ctrl.Result{}, Err: err, needUpdate: false}
	}

	var newConfigMap map[string]interface{}
	if err := json.Unmarshal(newJsonData, &newConfigMap); err != nil {
		return KcompareResult{Result: ctrl.Result{}, Err: err, needUpdate: false}
	}

	// Compare the two maps using reflect.DeepEqual
	if reflect.DeepEqual(existingConfigMap, newConfigMap) {
		fmt.Println("ConfigMap already up to date")
		return KcompareResult{Result: ctrl.Result{}, Err: nil, needUpdate: false}
	}
	fmt.Println("ConfigMap need an up date")

	return KcompareResult{Result: ctrl.Result{}, Err: nil, needUpdate: true}
}
