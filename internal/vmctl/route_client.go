package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
)

func (c *Client) ResolveComputerVersionRoute(ctx context.Context, routeSlotID string) (RouteResolution, error) {
	if c == nil || c.httpClient == nil {
		return RouteResolution{}, fmt.Errorf("vmctl client: route client is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, _, err := routeledger.ParseRouteSlotID(routeSlotID); err != nil {
		return RouteResolution{}, err
	}
	endpoint := ResolveComputerVersionRouteEndpoint(c.baseURL) + "?route_slot_id=" + url.QueryEscape(routeSlotID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return RouteResolution{}, fmt.Errorf("vmctl client: create ComputerVersion route request: %w", err)
	}
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return RouteResolution{}, fmt.Errorf("vmctl client: ComputerVersion route call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return RouteResolution{}, fmt.Errorf("vmctl client: read ComputerVersion route response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var errResp vmctlErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return RouteResolution{}, fmt.Errorf("vmctl client: ComputerVersion route resolution failed (status %d): %s", resp.StatusCode, errResp.Error)
		}
		return RouteResolution{}, fmt.Errorf("vmctl client: ComputerVersion route resolution failed with status %s", resp.Status)
	}
	var resolution RouteResolution
	if err := json.Unmarshal(body, &resolution); err != nil {
		return RouteResolution{}, fmt.Errorf("vmctl client: decode ComputerVersion route response: %w", err)
	}
	if err := validateRouteResolution(routeSlotID, resolution); err != nil {
		return RouteResolution{}, err
	}
	return resolution, nil
}

func validateRouteResolution(routeSlotID string, resolution RouteResolution) error {
	if !resolution.Slot.Current.Valid() || resolution.Slot.ID != routeSlotID ||
		resolution.LatestReceipt.RouteSlotID != routeSlotID ||
		resolution.LatestReceipt.ID != resolution.Slot.LatestReceiptID ||
		resolution.LatestReceipt.CommittedGeneration != resolution.Slot.Generation ||
		!routeledger.SameVersion(resolution.Slot.Current, resolution.LatestReceipt.New) ||
		resolution.CodeClosure.Ref != resolution.Slot.Current.CodeRef ||
		resolution.ArtifactProgram.Ref != resolution.Slot.Current.ArtifactProgramRef {
		return fmt.Errorf("vmctl client: ComputerVersion route response failed authority joins")
	}
	if err := resolution.CodeClosure.Verify(); err != nil {
		return fmt.Errorf("vmctl client: ComputerVersion route CodeRef failed verification: %w", err)
	}
	if err := resolution.ArtifactProgram.Verify(); err != nil {
		return fmt.Errorf("vmctl client: ComputerVersion route ArtifactProgramRef failed verification: %w", err)
	}
	return nil
}

func (c *Client) PinComputerVersionCode(ctx context.Context, closure computerversion.CodeClosure) (computerversion.CodeClosure, error) {
	var pinned computerversion.CodeClosure
	if err := c.postComputerVersionControl(ctx, PinComputerVersionCodeEndpoint(c.baseURL), closure, &pinned); err != nil {
		return computerversion.CodeClosure{}, err
	}
	if pinned.Ref != closure.Ref || pinned.Verify() != nil {
		return computerversion.CodeClosure{}, fmt.Errorf("vmctl client: pinned CodeRef response failed verification")
	}
	return pinned, nil
}

func (c *Client) PinComputerVersionArtifactProgram(ctx context.Context, program computerversion.ArtifactProgram) (computerversion.ArtifactProgram, error) {
	var pinned computerversion.ArtifactProgram
	if err := c.postComputerVersionControl(ctx, PinComputerVersionArtifactProgramEndpoint(c.baseURL), program, &pinned); err != nil {
		return computerversion.ArtifactProgram{}, err
	}
	if pinned.Ref != program.Ref || pinned.Verify() != nil {
		return computerversion.ArtifactProgram{}, fmt.Errorf("vmctl client: pinned ArtifactProgramRef response failed verification")
	}
	return pinned, nil
}

func (c *Client) TransitionComputerVersionRoute(ctx context.Context, command routeledger.TransitionCommand) (RouteResolution, error) {
	var resolution RouteResolution
	if err := c.postComputerVersionControl(ctx, TransitionComputerVersionRouteEndpoint(c.baseURL), command, &resolution); err != nil {
		return RouteResolution{}, err
	}
	if err := validateRouteResolution(strings.TrimSpace(command.RouteSlotID), resolution); err != nil {
		return RouteResolution{}, err
	}
	if resolution.TransitionReceipt == nil || !routeledger.ReceiptMatchesCommand(*resolution.TransitionReceipt, command) {
		return RouteResolution{}, fmt.Errorf("vmctl client: route transition receipt failed command join")
	}
	return resolution, nil
}

func (c *Client) postComputerVersionControl(ctx context.Context, endpoint string, input, output any) error {
	if c == nil || c.httpClient == nil {
		return fmt.Errorf("vmctl client: route client is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("vmctl client: encode ComputerVersion control request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("vmctl client: create ComputerVersion control request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vmctl client: ComputerVersion control call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("vmctl client: read ComputerVersion control response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var errResp vmctlErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return fmt.Errorf("vmctl client: ComputerVersion control failed (status %d): %s", resp.StatusCode, errResp.Error)
		}
		return fmt.Errorf("vmctl client: ComputerVersion control failed with status %s", resp.Status)
	}
	if err := json.Unmarshal(body, output); err != nil {
		return fmt.Errorf("vmctl client: decode ComputerVersion control response: %w", err)
	}
	return nil
}
