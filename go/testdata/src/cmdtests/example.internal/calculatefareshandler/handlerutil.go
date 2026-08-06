package calculatefareshandler

import (
	"context"
	"fmt"
	"time"

	"example.internal/fares/common/util/observabilityutil/tagkeys"

	"example.internal/fares/common/util/observabilityutil"
	"github.com/uber-go/tally"
	"go.uber.org/zap"
)

// this handlerutil comes from service-0016,
// make this simplified and service neutral for common service usages

// ExecuteHandlerFunc ...
type ExecuteHandlerFunc func(context.Context, *zap.Logger, *observabilityutil.EndpointLogContext, tally.Scope) (interface{}, error)

// Params ...
type Params struct {
	Handler     ExecuteHandlerFunc
	Logger      *zap.Logger
	Scope       tally.Scope
	Environment string
	Request     interface{}
	PanicError  error
}

// ExecuteHandler ...
func ExecuteHandler(ctx context.Context, p Params) (err error) {
	procedure := observabilityutil.GetProcedureContext(ctx)
	if procedure == observabilityutil.NoProcedureContext {
		ctx = observabilityutil.InitializeContext(ctx)
		procedure = observabilityutil.GetProcedureContext(ctx)
	}
	// sanitize from Ride::fareEstimateV2 to fareestimatev2 (convention on logger name ridehandler.fareestimatev2)
	method := observabilityutil.SanitizeProcedureFunction(procedure)
	// add standardized log context from ctx, header, request, response
	localLogger := *observabilityutil.LoggerWithContext(ctx, p.Logger).Named(method)
	localLogger = *observabilityutil.LoggerWithHeadersCompact(ctx, &localLogger)
	localLogger = *observabilityutil.LoggerWithRequestCompact(&localLogger, p.Request)
	scope := observabilityutil.ScopeWithContext(ctx, p.Scope, method)
	ctx = observabilityutil.SetShadowContext(ctx, p.Environment)
	// add standardized log context from EndpointLogContext{} - ex, from specific request data
	// p.Handler() could add endpoint/biz specific log context - non standardized log context
	customLogContext := observabilityutil.EndpointLogContext{}
	// emit metic, log
	defer observabilityutil.TrackMethod(&localLogger, scope, time.Now(), &err, observabilityutil.SucessOperationInfo)
	// set CustomLogContext in Handler() and set it to logger before emitting in TrackMethod()
	defer func() {
		localLogger = *observabilityutil.LoggerWithEndpointLogContext(&localLogger, &customLogContext)
	}()
	// panic recovery
	defer func() {
		if r := recover(); r != nil {
			localLogger.Error("Recovered from panic. Returning internal server error",
				observabilityutil.ZapReflect(tagkeys.Recover, r),
				zap.Stack(tagkeys.Stack),
			)
			if p.PanicError != nil {
				err = p.PanicError
			} else {
				err = fmt.Errorf("Panic Error - %v", r)
			}
		}
	}()

	resp, err := p.Handler(ctx, &localLogger, &customLogContext, scope)
	if err != nil {
		return err
	}
	localLogger = *observabilityutil.LoggerWithResponseCompact(&localLogger, resp)
	return nil
}
