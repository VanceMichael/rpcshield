package calculatefareshandler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"example.internal/fares/common/rehearsalmiddlewares/actingmiddleware"
	"example.internal/fares/common/util/handlerutil"
	"example.internal/fares/common/util/observabilityutil"
	"example.internal/marketplace/coordinated-structural-pricing/lib/yarpcerrors"
	trechandler "example.internal/money/trec/handler/calculatefareshandler"
	"example.internal/pricing/wayfare/clients/service0045"
	"example.internal/pricing/wayfare/lib/api/eats/eatsutils/dealfinder"
	comparator "example.internal/pricing/wayfare/lib/api/eats/eatsutils/shadowcomparator/treccalculatefareresponsecomparator"
	"example.internal/pricing/wayfare/lib/api/eats/renderers/models"
	"example.internal/pricing/wayfare/lib/api/eats/renderers/trecview"
	"example.internal/pricing/wayfare/lib/api/eats/renderers/trecview/triptaxbuilder"
	sdkhandler "example.internal/pricing/wayfare/lib/api/eats/trec"
	trecutil "example.internal/pricing/wayfare/lib/api/eats/trec"
	triptax "example.internal/pricing/wayfare/lib/api/eatsdelivery/collectors/trip_tax"
	"example.internal/pricing/wayfare/lib/api/eatsdelivery/deliveryutil"
	libheaders "example.internal/pricing/wayfare/lib/headers"
	"example.internal/pricing/wayfare/lib/sdk"
	"example.internal/pricing/wayfare/lib/shadow"
	"example.internal/pricing/wayfare/loggers"
	"example.internal/rt/flipr-client-go.git/flipr"
	"github.com/uber-go/tally"
	"github.com/uber/tchannel-go/thrift"
	"go.uber.org/fx"
	"go.uber.org/yarpc"
	"go.uber.org/zap"
	"thriftrw/example.internal/everything/thrift-types/eats"
	"thriftrw/example.internal/money/trec/trec"
)

const (
	_calculateTransportTrip = "calculateTransportTrip"
	_calculateEatsOrder     = "calculateEatsOrder"
	// ShouldRenderCourierExclusiveTaxesFliprKey is the flipr key for courier exclusive tax rendering feature flag
	ShouldRenderCourierExclusiveTaxesFliprKey = "wayfare.calculatetransporttrip.should_render_exclusive_tax"
	// ShouldRenderCourierExclusiveTaxesEnvFliprKey is the flipr key for courier exclusive tax rendering feature flag by using env
	ShouldRenderCourierExclusiveTaxesEnvFliprKey = "wayfare.transporttrip.should_render_exclusive_tax_by_env"
)

const (
	_featureNameKey           = "featureName"
	_businessModelKey         = "businessModelType"
	_orderCategoryKey         = "orderCategory"
	_cityIDKey                = "cityID"
	_isBillSplitBySubtotalKey = "isBillSplitBySubtotal"
	_countryISO2              = "countryISO2"
	_durationKey              = "duration"
	_fareSessionKey           = "fareSessionUUID"
	_storeUUIDKey             = "storeUUID"
	_driverUUIDKey            = "driverUUID"
	_requestTimeInSec         = "requestTimeInSec"
	_orderStatus              = "orderStatus"
	_deliveryFraudStatus      = "deliveryFraudStatus"
)

// CalculateFaresHandler is an interface that calculate fare details for orderJob and rushJob.
type CalculateFaresHandler interface {
	CalculateEatsOrder(
		ctx context.Context,
		request *trec.CalculateEatsOrderRequest,
	) (*trec.CalculateFareResponse, error)
	CalculateTransportTrip(
		ctx context.Context,
		request *trec.CalculateTransportTripRequest,
	) (*trec.CalculateFareResponse, error)
}

// Params holds a list of parameters to construct CalculateFaresHandler.
type Params struct {
	fx.In

	TrecHandler       trechandler.CalculateFaresHandler
	SdkHandler        sdkhandler.CalculateFaresHandler
	ShadowManager     shadow.Manager
	DealFinder        dealfinder.DealFinder
	Logger            *zap.Logger
	FaresFlipr        flipr.FliprClient `name:"fares_flipr"`
	ComparatorFactory *comparator.Factory
	TripTaxBuilder    triptaxbuilder.Interface
	Service0045       service0045.Gateway
	Metric            tally.Scope
	EnvContext        envfx.Context
}

type handlerImpl struct {
	trecHandler       trechandler.CalculateFaresHandler
	sdkHandler        sdkhandler.CalculateFaresHandler
	shadowManager     shadow.Manager
	dealFinder        dealfinder.DealFinder
	rushJobRenderer   trecview.RushJobRenderer
	orderJobRenderer  trecview.OrderJobRenderer
	logger            *zap.Logger
	faresFlipr        flipr.FliprClient
	comparatorFactory *comparator.Factory
	tripTaxBuilder    triptaxbuilder.Interface
	service0045       service0045.Gateway
	metric            tally.Scope
	envContext        envfx.Context
}

// NewHandler creates a new instance of the handler.
func NewHandler(params Params) CalculateFaresHandler {
	return &handlerImpl{
		trecHandler:       params.TrecHandler,
		sdkHandler:        params.SdkHandler,
		shadowManager:     params.ShadowManager,
		dealFinder:        params.DealFinder,
		logger:            params.Logger,
		faresFlipr:        params.FaresFlipr,
		comparatorFactory: params.ComparatorFactory,
		tripTaxBuilder:    params.TripTaxBuilder,
		service0045:       params.Service0045,
		metric:            params.Metric,
		envContext:        params.EnvContext,
	}
}

func (h handlerImpl) CalculateEatsOrder(
	ctx context.Context,
	request *trec.CalculateEatsOrderRequest,
) (response *trec.CalculateFareResponse, err error) {

	err = handlerutil.ExecuteHandler(ctx, handlerutil.Params{
		Handler: func(
			ctx context.Context, endpointLogger *zap.Logger, logContext *observabilityutil.EndpointLogContext, metric tally.Scope,
		) (interface{}, error) {
			*logContext = observabilityutil.EndpointLogContext{
				WorkflowUUID: string(request.GetWorkflowUUID()),
			}
			response, err = h.calculateEatsOrderHandler(ctx, request)
			return response, err
		},
		Logger:      h.logger,
		Scope:       h.metric,
		Environment: h.envContext.Environment,
		Request:     request,
		PanicError: yarpcerrors.InternalServerError(
			fmt.Sprintf(
				"panic when shadowing %s for trip %s",
				_calculateEatsOrder,
				request.GetJobUUID(),
			),
		),
	})
	return response, err
}
func (h handlerImpl) calculateEatsOrderHandler(
	ctx context.Context,
	request *trec.CalculateEatsOrderRequest,
) (response *trec.CalculateFareResponse, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = yarpcerrors.InternalServerError(
				fmt.Sprintf(
					"panic when shadowing %s for trip %s : %v",
					_calculateEatsOrder,
					request.GetJobUUID(),
					p),
			)
		}
	}()
	actingmiddleware.AddFilteringVars(ctx, map[string]any{
		"tenancy": yarpc.CallFromContext(ctx).Header(libheaders.XUberTenancy),
	})

	// fetch driverFraudStatus from deliveryJobInfos
	hasFraudDeliveryJob := trecutil.HasFraudDeliveryJob(request.GetDeliveryJobInfos())

	tags, fields, shadowConstraints := h.buildMetricsTagsAndShadowConstraints(
		ctx,
		_calculateEatsOrder,
		request.GetWorkflowUUID().String(),
		request.GetCityID(),
		request.GetBusinessModelType(),
		request.GetOrderCategory(),
		request.GetRequestTime(),
		request.GetAgoraStoreUUID().String(),
		request.GetRushTripPartnerUUID().String(),
		request.GetStatus().String(),
		hasFraudDeliveryJob,
		request.GetIsBillSplitBySubtotal(),
	)

	shadowResult, err := h.shadowManager.Shadow(
		ctx,
		_calculateEatsOrder,
		h.calculateEatsOrderWithTrec,
		h.calculateEatsOrderWithSdk,
		request,
		h.trecShadowComparator(tags, fields...),
		shadowConstraints,
		nil,
		nil,
		zap.Stringer(_fareSessionKey, request.GetWorkflowUUID()),
	)

	if err != nil {
		return nil, err
	}

	response = shadowResult.(*trec.CalculateFareResponse)
	return response, nil
}

func (h handlerImpl) CalculateTransportTrip(
	ctx context.Context,
	request *trec.CalculateTransportTripRequest,
) (response *trec.CalculateFareResponse, err error) {

	err = handlerutil.ExecuteHandler(ctx, handlerutil.Params{
		Handler: func(
			ctx context.Context, endpointLogger *zap.Logger, logContext *observabilityutil.EndpointLogContext, metric tally.Scope,
		) (interface{}, error) {
			*logContext = observabilityutil.EndpointLogContext{
				WorkflowUUID: string(request.GetWorkflowUUID()),
			}
			response, err = h.calculateTransportTripHandler(ctx, request)
			return response, err
		},
		Logger:      h.logger,
		Scope:       h.metric,
		Environment: h.envContext.Environment,
		Request:     request,
		PanicError: yarpcerrors.InternalServerError(
			fmt.Sprintf(
				"panic when shadowing %s for trip %s",
				_calculateEatsOrder,
				request.GetJobUUID(),
			),
		),
	})
	return response, err
}
func (h handlerImpl) calculateTransportTripHandler(
	ctx context.Context,
	request *trec.CalculateTransportTripRequest,
) (response *trec.CalculateFareResponse, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = yarpcerrors.InternalServerError(
				fmt.Sprintf(
					"panic when shadowing %s for trip %s : %v",
					_calculateTransportTrip,
					request.GetJobUUID(),
					p),
			)
		}
	}()
	actingmiddleware.AddFilteringVars(ctx, map[string]any{
		"tenancy": yarpc.CallFromContext(ctx).Header(libheaders.XUberTenancy),
	})

	tags, fields, shadowConstraints := h.buildMetricsTagsAndShadowConstraints(
		ctx,
		_calculateTransportTrip,
		request.GetWorkflowUUID().String(),
		request.GetCityID(),
		request.GetBusinessModelType(),
		request.GetOrderCategory(),
		request.GetRequestTime(),
		"",
		request.GetDriverUUID().String(),
		request.GetStatus().String(),
		trecutil.IsFlaggedTrip(request.GetDriverFraudStatus()),
		false,
	)
	shadowResult, err := h.shadowManager.Shadow(
		ctx,
		_calculateTransportTrip,
		h.calculateTransportTripWithTrec,
		h.calculateTransportTripWithSdk,
		request,
		h.trecShadowComparator(tags, fields...),
		shadowConstraints,
		nil,
		nil,
		zap.Stringer(_fareSessionKey, request.GetWorkflowUUID()),
	)

	if err != nil {
		return nil, err
	}

	response = shadowResult.(*trec.CalculateFareResponse)
	return response, nil
}

// calculateEatsOrderWithTrec process CalculateEatsOrderRequest by invoking legacy Trec handler.
func (h *handlerImpl) calculateEatsOrderWithTrec(ctx context.Context, req interface{}) (interface{}, error) {
	request := req.(*trec.CalculateEatsOrderRequest)

	headers := getHeadersFromContext(ctx)
	// we need to wrap the native context into a thrift context because the legacy tchannel handler requires it.
	return h.trecHandler.CalculateEatsOrder(thrift.WithHeaders(ctx, headers), request)
}

// calculateTransportTripWithTrec process CalculateTransportTripRquest by invoking legacy Trec handler.
func (h *handlerImpl) calculateTransportTripWithTrec(ctx context.Context, req interface{}) (interface{}, error) {
	request := req.(*trec.CalculateTransportTripRequest)

	headers := getHeadersFromContext(ctx)
	// we need to wrap the native context into a thrift context because the legacy tchannel handler requires it.
	response, err := h.trecHandler.CalculateTransportTrip(thrift.WithHeaders(ctx, headers), request)
	if err != nil {
		return response, err
	}

	territoryInfo, err := h.service0045.GetTerritory(ctx, request.GetCityID())

	if err != nil {
		h.logger.Error("Error calling Service0045 Client",
			zap.String(_fareSessionKey, request.GetWorkflowUUID().String()),
			zap.Error(err))
		return response, err
	}

	countryISO2 := territoryInfo.GetCountryISO2()

	// Render courier exclusive taxes and set the trec.CalculateFareResponse.taxes if present.
	// We are migrating from inclusive tax adjustments to exclusive tax adjustments. As trec.CalculateTransportTrip
	// forwards the tax information to OJ and it is being migrated to wayfare, we are adding the rendering
	// logic for exclusive taxes here under TrecResponseView. The same renderer will be used post trec deprecation in sdkHandler.CalculateTransportTrip
	if !shouldRenderCourierExclusiveTaxes(
		request.GetBusinessModelType(),
		request.GetStatus(),
		request.GetOrderCategory(),
		request.GetDriverUUID(),
		request.GetCityID(),
		countryISO2,
		request.GetWorkflowUUID().String(),
		h.faresFlipr) {
		return response, err
	}
	currentDeal, e := h.dealFinder.ReadCurrentDeliveryFinalizeDealV3(ctx, request.GetWorkflowUUID().String(), []string{request.GetJobUUID()})
	if e != nil || currentDeal == nil {
		h.logger.Error("Error reading the current delivery finalize deal",
			zap.Stringer("workflowUUID", request.GetWorkflowUUID()),
			zap.String("jobUUID", request.GetJobUUID()),
			zap.Error(e))
		return response, e
	}

	fare := sdk.Fare(*currentDeal.Fare)

	env := currentDeal.MakeEnv(ctx, h.logger, loggers.SamplingLogger(h.logger))
	fr := env.FindFareRequestByUUID(fare.FareRequestUUID)
	fareMarketData := deliveryutil.FareMarketData(*fr.MarketData)

	fliprConstraints := triptax.GetFliprConstraints(fareMarketData, *env)
	if !shouldRenderCourierExclusiveTaxesByEnv(fliprConstraints, h.faresFlipr) {
		return response, err
	}

	// tripTaxBuilder will return courier exclusive tax
	tripTaxes, err := h.tripTaxBuilder.Render(&fare, models.JobTypeRushJob)

	if len(tripTaxes) != 0 && response != nil {
		response.Taxes = tripTaxes
	}

	return response, err
}

func getHeadersFromContext(ctx context.Context) map[string]string {
	// Extracting the headers explicitly as thrift.Wrap [tchannel] does not work well with yarpc context for headers
	headers := make(map[string]string)
	call := yarpc.CallFromContext(ctx)
	headerNames := call.HeaderNames()
	for _, headerName := range headerNames {
		headers[headerName] = call.Header(headerName)
	}
	return headers
}

// calculateEatsOrderWithSdk process CalculateEatsOrderRequest by calling new sdk handler.
func (h *handlerImpl) calculateEatsOrderWithSdk(ctx context.Context, req interface{}) (interface{}, error) {
	request := req.(*trec.CalculateEatsOrderRequest)
	return h.sdkHandler.CalculateEatsOrder(ctx, request)
}

// calculateTransportTripWithSdk process CalculateTransportTripRequest by calling new sdk handler.
func (h *handlerImpl) calculateTransportTripWithSdk(ctx context.Context, req interface{}) (interface{}, error) {
	request := req.(*trec.CalculateTransportTripRequest)
	return h.sdkHandler.CalculateTransportTrip(ctx, request)
}

func (h *handlerImpl) trecShadowComparator(tags map[string]string, fields ...zap.Field) shadow.ResponsesComparator {
	return func(ctx context.Context,
		_ string,
		_, oldResponse, newResponse interface{},
		_ tally.Scope,
		_ *zap.Logger,
	) bool {
		// Unfortunately in order to still use the existing shadow manager, we have to
		// give up comparing the errors for now;
		oldResult := comparator.Result{
			Response: oldResponse.(*trec.CalculateFareResponse),
		}
		newResult := comparator.Result{
			Response: newResponse.(*trec.CalculateFareResponse),
		}

		shadowComparator := h.comparatorFactory.NewComparator(
			comparator.WithScopeTags(tags),
			comparator.WithLogFields(fields...),
		)
		return shadowComparator.Compare(ctx, oldResult, newResult)
	}
}

func (h *handlerImpl) buildMetricsTagsAndShadowConstraints(
	ctx context.Context,
	featureName, workflowUUID string,
	cityID int32,
	businessModelType eats.BusinessModelType,
	orderCategory eats.OrderCategory,
	requestTime trec.Timestamp,
	storeUUID string,
	driverUUID string,
	status string,
	fraudStatus bool,
	isBillSplitBySubtotal bool,
) (map[string]string, []zap.Field, flipr.Constraints) {
	requestAt := time.Unix(int64(requestTime)/1000, 0)
	duration := time.Since(requestAt).Hours()

	tags := map[string]string{
		_featureNameKey:   featureName,
		_cityIDKey:        string(cityID),
		_businessModelKey: businessModelType.String(),
		_orderCategoryKey: orderCategory.String(),
	}

	fields := []zap.Field{
		zap.String(_featureNameKey, featureName),
		zap.Int32(_cityIDKey, cityID),
		zap.Stringer(_businessModelKey, businessModelType),
		zap.Stringer(_orderCategoryKey, orderCategory),
		zap.String(_fareSessionKey, workflowUUID),
		zap.Bool(_deliveryFraudStatus, fraudStatus),
		zap.Bool(_isBillSplitBySubtotalKey, isBillSplitBySubtotal),
	}

	shadowConstraints := map[string]interface{}{
		_cityIDKey:                cityID,
		_businessModelKey:         businessModelType.String(),
		_orderCategoryKey:         orderCategory.String(),
		_durationKey:              duration,
		_storeUUIDKey:             storeUUID,
		_driverUUIDKey:            driverUUID,
		_requestTimeInSec:         int64(requestTime) / 1000,
		_orderStatus:              status,
		_deliveryFraudStatus:      strconv.FormatBool(fraudStatus),
		_isBillSplitBySubtotalKey: strconv.FormatBool(isBillSplitBySubtotal),
		_fareSessionKey:           workflowUUID,
	}

	// Here we call service0045 to fetch countryISO2; This is not strictly necessary but countryISO is helpful in the rollout
	// phase especially in the markets whose business model is not explicit, such as US, Japan and Brazil; Without countryISO2,
	// we'd have to rely on cityID and it is going to be difficult to roll out globally using cityID.
	territoryInfo, err := h.service0045.GetTerritory(ctx, cityID)

	// Ideally we should throw error for this, but it will introduce a major dependency in Service0045
	// For now we will only log this error
	if err != nil {
		h.logger.Warn("Error calling Service0045 Client",
			zap.String("featureName", featureName),
			zap.String("workflowUUID", workflowUUID),
			zap.Error(err))
	}

	countryISO2 := territoryInfo.GetCountryISO2()
	fields = append(fields, zap.String(_countryISO2, countryISO2))
	tags[_countryISO2] = countryISO2
	shadowConstraints[_countryISO2] = countryISO2

	return tags, fields, []flipr.Constraint{flipr.Constraints2(shadowConstraints)}
}

func shouldRenderCourierExclusiveTaxes(
	businessModelType eats.BusinessModelType,
	tripStatus trec.TripStatus,
	orderCategory eats.OrderCategory,
	driverUUID trec.UUID, cityID int32,
	countryISO2 string,
	workflowUUID string,
	faresFlipr flipr.FliprClient,
) bool {
	shouldRender, _ := faresFlipr.GetBoolValue(
		context.Background(),
		ShouldRenderCourierExclusiveTaxesFliprKey,
		[]flipr.Constraint{
			flipr.Constraints2(map[string]interface{}{
				"businessModelType": businessModelType.String(),
				"orderCategory":     orderCategory.String(),
				"driverUUID":        driverUUID.String(),
				"tripStatus":        tripStatus.String(),
				_cityIDKey:          cityID,
				_countryISO2:        countryISO2,
				_fareSessionKey:     workflowUUID,
			}),
		},
		false,
	)
	return shouldRender
}

// If shouldRenderCourierExclusiveTaxesByEnv return false it means exclusive taxes should not be rendered for this region
// The purpose of gating trip taxes with additional flipr is shouldRenderCourierExclusiveTaxes is created using trec's request
// but we want to use farestore's data to render taxes in selected countries. Because of limitation, everytime we will add a new country in
// the internal feature-flag config to enable trip tax we should block rendering using this flipr
func shouldRenderCourierExclusiveTaxesByEnv(constraints flipr.Constraints, faresFlipr flipr.FliprClient) bool {
	shouldRender, _ := faresFlipr.GetBoolValue(
		context.Background(),
		ShouldRenderCourierExclusiveTaxesEnvFliprKey,
		constraints,
		false,
	)
	return shouldRender
}
