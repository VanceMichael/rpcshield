func (c *controller) Charge(
	ctx context.Context,
	request *entity.ChargeRequest,
) (*entity.ChargeResponse, error) {
	tenancy := c.tenancyLib.GetTenancyUsingClient(ctx)
	// if either Payer or PaymentProfile are nil, guarantee they are populated from PaymentAccount
	// PaymentAccountUUID is guaranteed to be present from validation if these aren't

	var paymentProfileOwnerUUID uuid.UUID
	var homeCurrencyCode *string
	var paymentAccountType *entity.NinaPaymentAccountType
	var paymentAccountInfo *entity.GetPaymentAccountInfoResponse
	var paymentAccountOwnerUUID uuid.UUID
	var entityType *entity.NinaAccountEntityType

	authFlowType := utils.GetAuthFlowType(request.AuthChargeAction)

	if request.PaymentAccountUUID != nil {
		response, err := c.dataaggregation.GetPaymentAccountInfo(ctx,
			&entity.GetPaymentAccountInfoRequest{
				PaymentAccountUUID: request.PaymentAccountUUID,
			},
		)
		if err != nil {
			return nil, err
		}
		if request.PayerUUID == nil {
			request.PayerUUID = ptr.UUID(response.PayerUUID)
		}
		if request.PaymentProfileUUID == nil {
			request.PaymentProfileUUID = ptr.UUID(response.PaymentProfileUUID)
		}
		paymentProfileOwnerUUID = response.PaymentProfileOwnerUUID
		if !utils.IsAnyUUIDNilInArray(response.BackingPaymentProfileOwnerUUID, request.PaymentProfileUUID, response.BackingPaymentProfileUUID) &&
			utils.AreUUIDsEqual(*request.PaymentProfileUUID, *response.BackingPaymentProfileUUID) {
			isBackingPaymentProfileOwnerEnabled, err := c.businessconfigc.IsBackingPaymentProfileOwnerEnabled(ctx, *request.PayerUUID)
			if err != nil {
				return nil, err
			}
			if isBackingPaymentProfileOwnerEnabled {
				logger.FromContext(ctx).Info("Using backingpp owner for charge request",
					zap.Stringer(constants.PayerUUID, request.PayerUUID),
					zap.Stringer(constants.PaymentProfileUUID, request.PaymentProfileUUID),
					zap.Stringer(constants.PaymentProfileOwnerUUID, response.BackingPaymentProfileOwnerUUID))
				paymentProfileOwnerUUID = *response.BackingPaymentProfileOwnerUUID
			}
		}
		if safe.String(response.HomeCurrencyCode) == "" {
			metrics.FromContext(ctx).Tagged(map[string]string{
				"tenancy": tenancy,
			}).Counter("charge.payment.account.without.home.currency").Inc(1)

			logger.FromContext(ctx).Warn(
				"Home currency missing for organisation",
				zap.String("organisation_uuid", response.OwnerUUID.String()),
				zap.String("tenancy", safe.String(response.Tenancy)),
				zap.String(constants.PaymentProfileUUID, request.PaymentProfileUUID.String()),
			)
		} else {
			homeCurrencyCode = response.HomeCurrencyCode
		}
		paymentAccountType = response.PaymentAccountType
		paymentAccountOwnerUUID = response.OwnerUUID
		entityType = response.PaymentAccountEntityType
		paymentAccountInfo = response
	}
	if c.enableMinimumThresholdTraffic(ctx, request.PayerUUID, determineOriginator(ctx, request.Originator)) && paymentAccountType != nil {
		err := c.validateMinimumThreshold(ctx, request.Amount, *paymentAccountType)
		if err != nil {
			logger.FromContext(ctx).Info("Error from validateMinimumThreshold",
				zap.Any("Error", err),
				zap.Any("Request", request))
			return nil, err
		}
	}
	// At this point, we are sure PayerUUID is not nil, so if PaymentAccountUUID wasn't supplied and
	// we couldn't find the PaymentProfileOwnerUUID from the PaymentAccount, assume PayerUUID owns the
	// payment profile
	if paymentProfileOwnerUUID == uuid.Nil {
		paymentProfileOwnerUUID = *request.PayerUUID
	}
	logger := logger.FromContext(ctx).With(
		zap.Stringer("payerUUID", request.PayerUUID),
		zap.String("tenancy", tenancy),
	)
	scope := metrics.FromContext(ctx)
	enableClearinghouseTraffic := c.enableClearinghouseTraffic(ctx, request.PayerUUID, determineOriginator(ctx, request.Originator))
	isClearinghouseTrafficVal := isClearinghouseTraffic(ctx, request.BillType, enableClearinghouseTraffic, tenancy)
	if isClearinghouseTrafficVal {
		// PaymentAccountUUID cannot be nil for when Bill type is JOB, checked in handler
		isUnsupported, err := c.currencyIsUnsupported(
			ctx,
			request.Amount.CurrencyCode,
			*request.PaymentAccountUUID,
			paymentAccountType,
		)
		if err != nil {
			logger.Warn("Error Unsupported Currency check gave an error",
				zap.Stringer("tripUUID", request.JobUUID),
				zap.Stringer("sourcePaymentUUID", request.SourcePaymentUUID),
				zap.String("currencyCode", request.Amount.CurrencyCode),
				zap.String("tenancy", tenancy),
			)
			scope.Tagged(map[string]string{
				"tenancy":              tenancy,
				constants.AuthFlowType: authFlowType,
			}).Counter("charge.unsupported.currency.check.error").Inc(1)
			return nil, err
		}
		if isUnsupported {
			logger.Warn("Error Currency is not supported",
				zap.Stringer("tripUUID", request.JobUUID),
				zap.Stringer("sourcePaymentUUID", request.SourcePaymentUUID),
				zap.String("currencyCode", request.Amount.CurrencyCode),
				zap.String("tenancy", tenancy),
			)
			scope.Tagged(map[string]string{
				"tenancy":              tenancy,
				constants.AuthFlowType: authFlowType,
			}).Counter("charge.currency.unsupported.error").Inc(1)
			return nil, ErrCurrencyUnsupportedFailed
		}
		if paymentAccountInfo == nil || paymentAccountInfo.Status == nil || *paymentAccountInfo.Status != entity.NinaPaymentAccountStatusActive {
			logger.Warn("Error Payment Account is Inactive",
				zap.Stringer("tripUUID", request.JobUUID),
				zap.Stringer("sourcePaymentUUID", request.SourcePaymentUUID),
				zap.Any("paymentAccountUUID", request.PaymentAccountUUID),
				zap.String("tenancy", tenancy),
			)
			scope.Tagged(map[string]string{
				"tenancy":              tenancy,
				constants.AuthFlowType: authFlowType,
			}).Counter("charge.payment.account.inactive").Inc(1)
			return nil, ErrPaymentAccountInactive
		}
		if config.AppCfg.Configuration.EnrichTripEnabled &&
			paymentAccountInfo.PaymentAccountEntityType != nil &&
			*paymentAccountInfo.PaymentAccountEntityType == entity.NinaAccountEntityTypeInitechOrganization &&
			!c.isTripAttributionWorkflowDisabled(ctx, request.PayerUUID, request.JobType) {
			err = c.cadence.EnqueueTripAttributionWorkflow(ctx, entity.TripAttributionWorkflowParams{
				TripUUID:                *request.JobUUID,
				PaymentAccountOwnerUUID: paymentAccountInfo.OwnerUUID,
				ClientUUID:              request.ClientUUID,
				SourcePaymentUUID:       *request.SourcePaymentUUID,
				Operation:               entity.CapitalBillOperationCharge,
			})
			if err != nil {
				logger.Warn("Error EnqueueTripAttributionWorkflow failed",
					zap.Stringer("tripUUID", request.JobUUID),
					zap.Stringer("sourcePaymentUUID", request.SourcePaymentUUID),
					zap.String("tenancy", tenancy),
				)
				scope.Tagged(map[string]string{
					"tenancy":              tenancy,
					constants.AuthFlowType: authFlowType,
				}).Counter("charge.enqueue.trip.attribution.failed").Inc(1)
				return nil, err
			}
		}

		paymentAccountTenancy := ""
		if paymentAccountInfo != nil && paymentAccountInfo.Tenancy != nil {
			paymentAccountTenancy = *paymentAccountInfo.Tenancy
		}
		logger.Info("Payment Account Tenancy",
			zap.Stringer("tripUUID", request.JobUUID),
			zap.Stringer("sourcePaymentUUID", request.SourcePaymentUUID),
			zap.String("paymentAccountTenancy", paymentAccountTenancy),
			zap.String("tenancy", tenancy),
		)
		if isShadowTrafficWithProductionEntities(tenancy, paymentAccountInfo) {
			logger.Info(
				"Settled without processing due to mismatch in payment_account tenancy and request tenancy",
				zap.Any("paymentAccount", paymentAccountInfo),
				zap.Any("capitalBillUUID", constants.DummyCapitalBillUUID.String()),
			)
			scope.Tagged(map[string]string{
				"tenancy":              tenancy,
				constants.AuthFlowType: authFlowType,
			}).Counter("charge.job.is.u4b.payment.error").Inc(1)
			return &entity.ChargeResponse{
				CapitalBillUUID: ptr.UUID(constants.DummyCapitalBillUUID),
			}, nil
		}

		// JobUUID and SourcePaymentUUID are present if the charge is of type `JOB`
		err = c.checkJobIsU4BPayment(ctx, *request.JobUUID, *request.SourcePaymentUUID, authFlowType)
		if err != nil {
			logger.Info("Job Is U4B Payment gives an error",
				zap.Stringer("tripUUID", request.JobUUID),
				zap.Stringer("sourcePaymentUUID", request.SourcePaymentUUID),
				zap.String("tenancy", tenancy),
				zap.Error(err),
			)
			scope.Tagged(map[string]string{
				"tenancy":              tenancy,
				constants.AuthFlowType: authFlowType,
			}).Counter("charge.job.is.u4b.payment.error").Inc(1)
			return nil, err
		}
		logger.Info("Clearinghouse logic successful")
	}

	// Perform a credit reversal if charge is triggered in response to a Payout adjustment
	if isPayout(&request.BillType) {
		err := utils.ValidateFieldsNonNilPointers(
			entity.Field{T: request.PayerUUID, N: constants.PropertyPayerUUID},
			entity.Field{T: request.PaymentAccountUUID, N: constants.PropertyPaymentAccountUUID},
			entity.Field{T: request.JobUUID, N: constants.PropertyJobUUID},
			entity.Field{T: request.SourcePaymentUUID, N: constants.PropertySourcePaymentUUID})

		if err != nil {
			return nil, err
		}

		defaultNote := constants.OrderManagementAdjustmentInfoDefaultNote
		creditReversalResponse, err := c.CreditReversal(ctx, &entity.CreditReversalRequest{
			PayerUUID:          *request.PayerUUID,
			PaymentAccountUUID: *request.PaymentAccountUUID,
			Amount:             request.Amount,
			Note:               request.Note,
			Originator:         request.Originator,
			JobUUID:            *request.JobUUID,
			SourcePaymentUUID:  *request.SourcePaymentUUID,
			TransactionNotes: entity.TransactionNotes{
				ExternalNote: defaultNote,
				InternalNote: &defaultNote,
			},
		})
		if err != nil {
			return nil, err
		}
		return &entity.ChargeResponse{
			CapitalBillUUID: creditReversalResponse.CapitalBillUUID,
		}, nil
	}

	if c.customFeeRouteEnabled(ctx, request) {
		resp, err := c.HandleU4BCustomFeeCharge(ctx, request)

		if err != nil {
			// Do not log or emit error metric if request is not enabled to go through, based on flipr evaluation
			if !strings.Contains(err.Error(), "Flow is disallowed for capital bill type") {
				logger.Warn("Error in custom fee route for capital bill",
					zap.Error(err))
				scope := metrics.FromContext(ctx)
				scope.Tagged(map[string]string{
					constants.AuthFlowType: authFlowType,
				}).Counter("charge.custom_fare.error").Inc(1)
			}
		} else {
			return resp, nil
		}
	}

	// TODO: @anjali.goenka Remove post testing

	isUpfrontFeatureEnabled := c.checkIfUpfrontTrip(ctx, request.JobUUID, request.GulfstreamJobType, request.CountryISO2, request.ClientUUID, paymentAccountInfo)

	findDuplicateOrNewUUIDRequest := &entity.FindDuplicateOrNewUUIDRequest{
		PayerUUID:         *request.PayerUUID,
		SourcePaymentUUID: request.SourcePaymentUUID,
		RequestedBillUUID: request.CapitalBillUUID,
		JobUUID:           request.JobUUID,
	}

	capitalBillProfileType := ptr.String(constants.ProfileTypeManagedBusiness)
	// check if fareSplitVersion is enabled or payerUUID is whitelisted
	if request.BillType == entity.CapitalBillTypeJob && !isUpfrontFeatureEnabled {
		var err error
		orderAndTrip, err := c.orderandtrip.GetOrderAndTripWithOptions(ctx, &entity.GetOrderAndTripWithOptionsRequest{
			JobUUID: *request.JobUUID,
			TripOptions: &entity.TridentTripColumnOptions{
				IncludeMinion: ptr.Bool(true),
			},
		})
		if err != nil {
			return nil, err
		}
		// Note: populate clientUUID only incase of eats fare split trips where input clientUUID is a minion.
		// For eats fare split minions, a charge request will not have a corresponding auth hold for capture yet
		// except for primary clientUUUID of the trip
		if request.ClientUUID != nil {
			groupOrderBillGenerationLogicEnabled, err := c.businessconfigc.IsGroupOrderBillGenerationLogicEnabled(ctx, *request.JobUUID, *request.PayerUUID)
			if err != nil {
				return nil, err
			}

			if groupOrderBillGenerationLogicEnabled {
				if orderAndTrip.IsEatsFareSplit() {
					metrics.FromContext(ctx).Counter("is_eats_fare_split_minion_trip").Inc(1)
					logger.With(
						zap.Stringer("payerUUID", request.PayerUUID),
						zap.Stringer("jobUUID", request.JobUUID),
					)
					logger.Info("Is Eats fare split minion trip")

					findDuplicateOrNewUUIDRequest.MasterClientUUID = orderAndTrip.GetPrimaryRequestor()
					findDuplicateOrNewUUIDRequest.ClientUUID = request.ClientUUID
				}
			} else {
				if c.isEatsFareSplitTripMinion(ctx, request.JobUUID, *request.PayerUUID, request.BillType, request.ClientUUID, orderAndTrip) {
					findDuplicateOrNewUUIDRequest.ClientUUID = request.ClientUUID
				}
			}
		}
		// Set profile type to nil if service-0033 is a delegate eats service-0033
		if orderAndTrip.IsEats() && orderAndTrip.GetHydratedOrder().IsDelegateBookingOrder() {
			capitalBillProfileType = nil
		}
	}

	response, err := c.capitalbillc.FindDuplicateOrNewUUID(ctx, findDuplicateOrNewUUIDRequest)
	if err != nil {
		return nil, err
	}
	if response.DuplicateUUID != nil {
		if response.DuplicateCapitalBill != nil {
			var jobType entity.CapitalBillJobType
			if response.DuplicateCapitalBill.JobType != nil {
				jobType = *response.DuplicateCapitalBill.JobType
			}

			//Log & add metrics when a duplicateUUID is found in the flow
			logger.Info("Duplicate UUID found", zap.Any("DuplicateCapitalBillUUID", response.DuplicateUUID),
				zap.Any("request", request))
			scope.Tagged(
				map[string]string{
					"instrument_type":      response.DuplicateCapitalBill.InstrumentType.String(),
					"job_type":             jobType.String(),
					constants.AuthFlowType: authFlowType,
				}).Counter("charge.duplicate.capitalbill").Inc(1)
			//This should not happpen unless someone is manipulating with the orderUUID, this metric is to track and alert such use-cases
			// ref: tracked internally as a known edge case in duplicate-payment detection
			if request.SourcePaymentUUID != response.DuplicateCapitalBill.SourcePaymentUUID {
				scope.Tagged(
					map[string]string{
						"instrument_type":      response.DuplicateCapitalBill.InstrumentType.String(),
						"job_type":             jobType.String(),
						constants.AuthFlowType: authFlowType,
					}).Counter("charge.duplicate.capitalbill.fraud.error").Inc(1)
			}

			var authHoldInfo *entity.RestrictorCaptureAuthHoldInfo
			if request.JobUUID != nil {
				authHoldInfo = &entity.RestrictorCaptureAuthHoldInfo{
					JobUUID:                  *request.JobUUID,
					UberTransactionID:        request.UberTransactionID,
					PaymentAccountOwnerUUID:  &paymentAccountOwnerUUID,
					PaymentAccountEntityType: entityType,
					HoldAmount:               request.Amount,
					AuthChargeAction:         request.AuthChargeAction,
				}
			}

			if response.DuplicateCapitalBill.InstrumentType == entity.CapitalBillInstrumentTypeUCharge {
				if err = c.cadence.EnqueueChargeLedgerForCapitalBillWorkflow(ctx, *response.DuplicateUUID, authHoldInfo); err != nil {
					return nil, err
				}
			}
		} else {
			if err = c.cadence.EnqueueChargeCapitalBillWorkflow(ctx, *response.DuplicateUUID); err != nil {
				return nil, err
			}
		}
		var state *entity.CapitalBillState
		if isUpfrontFeatureEnabled && response.DuplicateCapitalBill != nil {
			state = &response.DuplicateCapitalBill.State
			logger.Info("UpfrontTransaction - sending inProgressState for duplicateUUID", zap.String("capitalBillState", state.String()))
		}

		return &entity.ChargeResponse{CapitalBillUUID: response.DuplicateUUID, CapitalBillState: state}, nil
	}
	capitalBillUUID := *response.NewUUID

	if utils.IsBulkGiftingJobType(request.JobType) {
		// fetch payment profile from original grantResponse
		logger.Info("Bulk gifting charge request",
			zap.Stringer(constants.JobType, request.JobType))
		profileToCharge, err := c.FetchProfileForChargeInBulkGifting(ctx, capitalBillUUID, request.PaymentProfileUUID, paymentAccountInfo)
		if err != nil {
			return nil, err
		}
		request.PaymentProfileUUID = profileToCharge
	}
	logger.Info("Payment profile to charge",
		zap.Stringer(constants.PaymentProfileUUID, request.PaymentProfileUUID),
		zap.Stringer(constants.PaymentProfileOwnerUUID, paymentProfileOwnerUUID),
		zap.Any(constants.Request, request))

	localCurrencyCode := request.Amount.CurrencyCode // For non-pcp trip, request.Amount is in local currency
	isPcpTrip := c.isPcpTrip(ctx, request, paymentAccountInfo)
	var fxRateLocalToPreferredCurrency *string
	var pcpFxRateResponse *entity.PcpExchangeRateInfo
	if isPcpTrip {
		pcpFxRateResponse, err = c.fetchFxRateFromHCP(ctx, *request.FxIdentifier)
		if err != nil {
			return nil, err
		}
		fxRateLocalToPreferredCurrency = pcpFxRateResponse.FxRate
		localCurrencyCode = *pcpFxRateResponse.LocalCurrencyCode
	}
	var paymentProfile *entity.BankemojiPaymentProfile
	var getPaymentProfileAndFxRatesResponse *entity.GetPaymentProfileAndFxRatesResponse

	useUnifiedFxRates, err := c.businessconfigc.UseUnifiedExchangeRates(ctx, request, paymentAccountInfo)
	if err != nil {
		return nil, err
	}

	if useUnifiedFxRates && request.BillType == entity.CapitalBillTypeJob {
		getPaymentProfileAndFxRatesResponse, err = c.cffConcurrencyCtrl.GetPaymentProfileAndFxRates(ctx, entity.GetPaymentProfileAndFxRatesRequest{
			PaymentProfileUUID: *request.PaymentProfileUUID,
			OwnerUUID:          paymentProfileOwnerUUID,
			JobUUID:            *request.JobUUID,
			JobCurrencyCode:    localCurrencyCode,
			HomeCurrencyCode:   homeCurrencyCode,
		})
		if err != nil {
			return nil, err
		}
		paymentProfile = &getPaymentProfileAndFxRatesResponse.PaymentProfile
	} else {
		paymentProfile, err = c.walletAPIGatewayController.GetPaymentProfile(
			ctx, &entity.BankemojiGetPaymentProfileRequest{
				PaymentProfileUUID: *request.PaymentProfileUUID,
				OwnerUUID:          ptr.UUID(paymentProfileOwnerUUID),
			},
		)
		if err != nil {
			return nil, err
		}
	}
	if paymentProfile.DeletedAt != nil {
		if isUpfrontFeatureEnabled {
			logger.Error("Upfront Trip taken on deleted payment profile",
				zap.Stringer(constants.PaymentProfileUUID, request.PaymentProfileUUID),
				zap.Int64("DeletedAt", safe.Int64(paymentProfile.DeletedAt)))
			return nil, ErrDeletedPaymentProfile
		}
		logger.Warn(
			"Trip taken on deleted payment profile",
			zap.Stringer(constants.PaymentProfileUUID, request.PaymentProfileUUID),
			zap.Int64("DeletedAt", safe.Int64(paymentProfile.DeletedAt)),
			zap.Stringer(constants.JobUUID, request.JobUUID))
	}
	// Ensure payer actually owns the payment profile
	if paymentProfile.OwnerUUID != paymentProfileOwnerUUID {
		// return generic error, don't expose the real problem
		return nil, bankemoji.NewErrPaymentProfileNotFound(nil, request.PaymentProfileUUID.String())
	}
	instrumentType, err := mapper.BankemojiTokenTypeToCapitalBillInstrumentType(
		paymentProfile.TokenType,
	)
	if err != nil {
		return nil, err
	}

	// Verify uber cash balance. If zero or if currency not equal to home currency for uber cash instrument,
	// update payment profile to backing payment profile
	if *instrumentType == entity.CapitalBillInstrumentTypeUberCash {

		// charge on backing payment profile instead if true
		shouldChargeBackingPaymentProfile, err := c.IsOperationOnBackingPaymentProfile(ctx, &entity.IsOperationOnBackingPaymentProfileRequest{
			PayerUUID:          *request.PayerUUID,
			PaymentProfileUUID: *request.PaymentProfileUUID,
			Amount:             request.Amount,
			Operation:          entity.CapitalBillOperationCharge,
			BillType:           request.BillType,
		})
		if err != nil {
			return nil, err
		}

		if shouldChargeBackingPaymentProfile {
			resp, err := c.GetBackingPaymentProfileForUberCash(ctx, *request.PayerUUID)
			if err != nil {
				return nil, err
			}
			// Set to backing payment profile only if found
			if resp != nil {
				logger.Info("Setting payment profile to backing payment profile")
				request.PaymentProfileUUID = &resp.PaymentProfileUUID
				instrumentType = &resp.InstrumentType
			}
		}
	}

	if *instrumentType == entity.CapitalBillInstrumentTypeUberTest && !utils.IsTestingTenancy(tenancy) {
		logger.Error("Non test-tenancy charge with Uber Test payment profile",
			zap.Stringer("paymentProfileInstrumentType", instrumentType),
		)
		return nil, ErrTenancyMismatch
	}

	err = validateCharge(request, *instrumentType)
	if err != nil {
		return nil, err
	}

	// NOTE: because PAB is not showing as USD we have to force it for Event and UCharge payments
	// More details are in T1693785 and T1992613
	typesToConvertPABToUSD := []string{
		entity.CapitalBillTypeEventPayment.String(),
		entity.CapitalBillTypeUChargePayment.String(),
		entity.CapitalBillTypeServiceFee.String(),
	}
	if contains.String(typesToConvertPABToUSD, request.BillType.String()) {
		if request.Amount.CurrencyCode == "PAB" {
			request.Amount.CurrencyCode = "USD"
			request.FXRateUSDToJob = ptr.String("1.000000")
		}
	}

	if request.BillType == entity.CapitalBillTypeNonProfitAdjustment {
		// only logging for now
		logger.Info("Creating Charge transaction for NPA org",
			zap.Any("request", request))
	}

	if request.BillType == entity.CapitalBillTypeUChargePayment {
		if request.LedgerAccountUUID == nil {
			request.LedgerAccountUUID, err = c.laasc.ValidateAccount(ctx, *request.PayerUUID)
			if err != nil {
				return nil, ErrNoLedgerAccountUUID
			}
		}
		if request.PayerUUID != nil && paymentAccountType != nil {
			ppoEnabled, _ := c.isPPOEnabled(
				ctx,
				*request.PayerUUID,
				*paymentAccountType,
				time.Now().UnixMilli())
			if ppoEnabled != nil && ppoEnabled.IsPPOEnabled {
				// Throw error if request billType is UCharge Payment and Payer is PPO enabled organization
				logger.Error("PPO enabled organization cannot charge UCharge Payment",
					zap.Any("request", request))
				metrics.FromContext(ctx).Tagged(map[string]string{
					constants.AuthFlowType: authFlowType,
				}).Counter("ppo_enabled_org_ucharge_payment_attempt").Inc(1)
				return nil, ErrPPOTransactionNotPaidFromPPO
			}
		}
		resp, err := c.ledger.ValidateUChargePaymentV2(ctx, &entity.ValidateUChargePaymentRequest{
			Amount:             request.Amount,
			PayerUUID:          *request.PayerUUID,
			LedgerAccountUUID:  *request.LedgerAccountUUID, // guaranteed present from validation for type
			InstrumentType:     *instrumentType,
			PaymentAccountUUID: request.PaymentAccountUUID,
		})
		if err != nil {
			return nil, err
		}
		if resp != nil && resp.PendingBills != nil && len(resp.PendingBills) > 0 {
			for _, bill := range resp.PendingBills {
				_, err := c.DropOutstandingCapitalBill(ctx, &entity.DropOutstandingCapitalBillRequest{
					BillUUID:      bill.UUID,
					IgnoreBacking: false,
				})
				if err != nil {
					logger.Error("Error in dropping bill",
						zap.Error(err),
						zap.Stringer("billUUID", bill.UUID),
					)
					metrics.FromContext(ctx).Tagged(map[string]string{
						constants.AuthFlowType: authFlowType,
					}).Counter("charge_drop_capital_bill_failed").Inc(1)
					return nil, err
				}
				logger.Info("Drop Successfull",
					zap.Stringer("billUUID", bill.UUID),
				)
			}
			metrics.FromContext(ctx).Tagged(map[string]string{
				constants.AuthFlowType: authFlowType,
			}).Counter("dropping_pending_bills_successful").Inc(1)
		}
	}

	var fxRateUSDToJob *string
	var fareQuantumJobCurrency *string
	var homeCurrencyExchangeRate *string
	var homeCurrencyInfo *entity.TincupCurrency

	if useUnifiedFxRates && request.BillType == entity.CapitalBillTypeJob {
		fxRateUSDToJob = &getPaymentProfileAndFxRatesResponse.JobExchangeRate.ExchangeRate
		fareQuantumJobCurrency = getPaymentProfileAndFxRatesResponse.JobCurrency.FareQuantum

		if getPaymentProfileAndFxRatesResponse.HomeExchangeRate != nil {
			homeCurrencyExchangeRate = &getPaymentProfileAndFxRatesResponse.HomeExchangeRate.ExchangeRate
		}

		if getPaymentProfileAndFxRatesResponse.HomeCurrency != nil {
			homeCurrencyInfo = getPaymentProfileAndFxRatesResponse.HomeCurrency
		}
	} else {
		// due to Validation layer, all types except UChargePayment and UChargeCredit must have FXRateUSDToJob supplied, so
		// use that rate, but retrieve for UChargePayment due to bug in supplied rate (D1386901)
		fxRateUSDToJob = request.FXRateUSDToJob
		fareQuantumJobCurrency = request.FareQuantumJobCurrency
		if request.BillType == entity.CapitalBillTypeUChargePayment ||
			request.BillType == entity.CapitalBillTypeUChargeCredit {
			exchangeRate, err := c.tincupc.GetExchangeRateWithFareQuantum(ctx, localCurrencyCode)
			if err != nil {
				return nil, err
			}
			fxRateUSDToJob = &exchangeRate.ExchangeRate
			fareQuantumJobCurrency = exchangeRate.FareQuantum
		}
		if fareQuantumJobCurrency == nil {
			currency, err := c.tincupg.GetCurrency(ctx, localCurrencyCode)
			if err != nil {
				return nil, err
			}
			fareQuantumJobCurrency = currency.FareQuantum
		}
	}

	var homeCurrencyResponse *entity.ConvertAmountToCurrencyResponse
	if !isPcpTrip {
		homeCurrencyResponse, err = c.dataaggregation.ConvertAmountToCurrency(ctx,
			&entity.ConvertAmountToCurrencyRequest{
				Amount:              request.Amount,
				FxRateUSDToExisting: *fxRateUSDToJob,
				FxRateUSDToDesired:  homeCurrencyExchangeRate,
				FareQuantumExisting: fareQuantumJobCurrency,
				DesiredCurrency:     homeCurrencyCode,
				DesiredCurrencyInfo: homeCurrencyInfo,
			},
		)
		if err != nil {
			return nil, err
		}
	} else {
		homeCurrencyResponse = &entity.ConvertAmountToCurrencyResponse{
			ConvertedAmount: &request.Amount,
		}
		if homeCurrencyExchangeRate != nil {
			homeCurrencyResponse.FxRateUSDToConverted = homeCurrencyExchangeRate
		}
		if homeCurrencyInfo != nil && homeCurrencyInfo.FareQuantum != nil {
			homeCurrencyResponse.FareQuantumConverted = homeCurrencyInfo.FareQuantum
		}
		if homeCurrencyCode != nil && (homeCurrencyResponse.FxRateUSDToConverted == nil || homeCurrencyResponse.FareQuantumConverted == nil) {
			currencyInfo, _ := c.tincupc.GetExchangeRateWithFareQuantum(ctx, *homeCurrencyCode)
			if currencyInfo != nil {
				homeCurrencyResponse.FareQuantumConverted = currencyInfo.FareQuantum
				homeCurrencyResponse.FxRateUSDToConverted = homeCurrencyExchangeRate
			}
		}
	}

	// Use non-pcp amounts as initial default values
	capitalBillAmount := request.Amount
	capitalBillHomeCurrencyAmount := homeCurrencyResponse.ConvertedAmount

	if isPcpTrip {
		// For PCP trip, request.Amount is in home/preferred currency
		capitalBillHomeCurrencyAmount = &request.Amount
		// Calculate the amount in local currency using fxRateLocalToPreferredCurrency
		logger.Info("Converting PCP amount from preferred to local currency",
			zap.String("fromCurrency", request.Amount.CurrencyCode),
			zap.Int64("fromAmount", request.Amount.Amount),
			zap.String("toCurrency", localCurrencyCode),
			zap.String("fxRate", *fxRateLocalToPreferredCurrency))
		var localCurrencyInfo *entity.TincupCurrency
		if getPaymentProfileAndFxRatesResponse != nil {
			localCurrencyInfo = &getPaymentProfileAndFxRatesResponse.JobCurrency
		}
		localCurrencyResponse, err := c.dataaggregation.ConvertToTargetCurrency(ctx,
			&entity.ConvertToTargetCurrencyRequest{
				Amount:                 &request.Amount,
				TargetCurrencyCode:     &localCurrencyCode,
				FxRateExistingToTarget: fxRateLocalToPreferredCurrency,
				TargetCurrencyInfo:     localCurrencyInfo,
			},
		)
		if err != nil {
			logger.Error("Failed to convert amount to local currency for PCP trip",
				zap.Error(err),
				zap.String("fromCurrency", request.Amount.CurrencyCode),
				zap.String("toCurrency", localCurrencyCode))
			return nil, err
		}
		capitalBillAmount = *localCurrencyResponse.ConvertedAmount
		logger.Info("Converted amount to local currency for PCP trip")
	}

	var resellerUUID *uuid.UUID
	if request.ResellerUUID != nil &&
		c.isIntegrationFeeEnabled(ctx, *request.PayerUUID) &&
		request.PayerUUID != request.ResellerUUID {
		// Hydrate ResellerUUID only if ResellerUUID != PayerUUID (skipping 1P trips)
		resellerUUID = request.ResellerUUID
	}

	if request.ClientUUID == nil && instrumentType != nil {
		instrumentTypeVal := *instrumentType
		logger.Info("Charge request miss client uuid",
			zap.Stringer("capitalBillUUID", capitalBillUUID),
			zap.Stringer("instrumentType", instrumentTypeVal),
		)
	}

	// Don't call restrictor Charge if paymentAccountOwnerUUID is uuid.Nil or entityType is nil or entityType is not Organization
	// TODO @Chirag : If error from flipr, we will not call restrictor Charge for now - Change logic to return error in future
	isAuthFliprEnabled := c.IsAuthHoldEnabled(ctx, paymentAccountOwnerUUID, entityType)
	// TODO @Chirag : Next line needs to be removed to call booster for PPT org when spend cap dimension is implemented
	isPeriodicOrg := instrumentType != nil && *instrumentType == entity.CapitalBillInstrumentTypeUCharge

	// If PSO Switch for Ucharge, call restrictor Validate Capping
	if isAuthFliprEnabled && isPeriodicOrg {
		_, wagErr := c.getPaymentProfileFromExtraPaymentData(ctx, paymentAccountOwnerUUID, request.ExtraPaymentData)
		tripType := utils.TripTypeFromPaymentProfileLookup(wagErr)
		tripTypeStr := string(tripType)

		// Resolve booster rollout ahead of time to gate fail-open vs fail-closed behavior consistently
		isBoosterRolloutEnabled := false
		if result, berr := c.isBoosterRolloutEnabled(ctx, paymentAccountOwnerUUID, func() *entity.NinaPaymentAccountType {
			if paymentAccountInfo != nil {
				return paymentAccountInfo.PaymentAccountType
			}
			return nil
		}(), "ValidateCapping", &tripTypeStr); berr != nil {
			logger.Warn("Failed to resolve booster rollout; defaulting to disabled", zap.Error(berr))
		} else {
			isBoosterRolloutEnabled = result
		}

		validateChargeForPaymentSwitchOrderRequest := &entity.ValidateChargeForPaymentSwitchOrderRequest{
			PaymentAccountOwnerUUID: paymentAccountOwnerUUID,
			PaymentExtensions:       request.PaymentExtensions,
			Amount:                  request.Amount,
			ConvertedAmount:         homeCurrencyResponse.ConvertedAmount,
			InstrumentType:          *instrumentType,
			JobUUID:                 request.JobUUID,
		}
		validateResult, err := c.ValidateChargeForPaymentSwitchOrder(ctx, validateChargeForPaymentSwitchOrderRequest)
		if err != nil {
			logger.Warn("ValidateCapping Error", zap.Error(err), zap.Bool("isBoosterRolloutEnabled", isBoosterRolloutEnabled))
			metrics.FromContext(ctx).Tagged(map[string]string{
				constants.AuthFlowType:    authFlowType,
				"isBoosterRolloutEnabled": strconv.FormatBool(isBoosterRolloutEnabled),
			}).Counter("charge.validate_capping.error").Inc(1)
			// If booster rollout is enabled, fail-closed; else fail-open
			if isBoosterRolloutEnabled {
				logger.Error("ValidateCapping Error", zap.Error(err), zap.Bool("isBoosterRolloutEnabled", isBoosterRolloutEnabled))
				return nil, err
			}
		} else {
			if validateResult {
				metrics.FromContext(ctx).Tagged(map[string]string{
					constants.AuthFlowType:    authFlowType,
					"isBoosterRolloutEnabled": strconv.FormatBool(isBoosterRolloutEnabled),
				}).Counter("charge.validate_capping.success").Inc(1)
			} else {
				// If booster rollout is enabled, reject; else fail-open
				if isBoosterRolloutEnabled {
					metrics.FromContext(ctx).Tagged(map[string]string{
						constants.AuthFlowType:    authFlowType,
						"isBoosterRolloutEnabled": strconv.FormatBool(isBoosterRolloutEnabled),
					}).Counter("charge.validate_capping.failed").Inc(1)
					logger.Error("ValidateCapping Failed",
						zap.Stringer("paymentAccountOwnerUUID", paymentAccountOwnerUUID),
						zap.Stringer("uuid", capitalBillUUID),
						zap.Bool("isBoosterRolloutEnabled", isBoosterRolloutEnabled))
					return nil, ErrValidateCappingError
				}
				// else fail-open: rollout disabled (shadow), proceed with old flow
			}
		}
	}

	capitalBill := &entity.CapitalBill{
		UUID:                           capitalBillUUID,
		PayerUUID:                      *request.PayerUUID,
		PaymentProfileOwnerUUID:        paymentProfileOwnerUUID,
		PaymentProfileUUID:             utils.MakeShardKey(*request.PaymentProfileUUID, paymentProfileOwnerUUID),
		Amount:                         capitalBillAmount,
		FXRateUSDToJob:                 *fxRateUSDToJob,
		FareQuantumJobCurrency:         fareQuantumJobCurrency,
		State:                          entity.CapitalBillStatePending,
		Operation:                      entity.CapitalBillOperationCharge,
		Type:                           request.BillType,
		InstrumentType:                 *instrumentType,
		SourcePaymentUUID:              request.SourcePaymentUUID,
		RelatedPaymentUUID:             request.RelatedPaymentUUID,
		JobUUID:                        request.JobUUID,
		PaymentAccountUUID:             request.PaymentAccountUUID,
		LedgerAccountUUID:              request.LedgerAccountUUID,
		CountryISO2:                    request.CountryISO2,
		HomeCurrencyCode:               homeCurrencyCode,
		HomeCurrencyAmount:             capitalBillHomeCurrencyAmount,
		FXRateUSDToHome:                homeCurrencyResponse.FxRateUSDToConverted,
		FareQuantumHomeCurrency:        homeCurrencyResponse.FareQuantumConverted,
		FXRateLocalToPreferredCurrency: fxRateLocalToPreferredCurrency,
		TaxInfo:                        request.TaxInfo,
		Note:                           request.Note,
		Payload:                        request.Payload,
		JobType:                        request.JobType,
		Originator:                     determineOriginator(ctx, request.Originator),
		Tenancy:                        &tenancy,
		BillingMode:                    ptr.String(constants.BillingModeCentralized),
		ProfileType:                    capitalBillProfileType,
		ResellerUUID:                   resellerUUID,
		ClientUUID:                     request.ClientUUID,
		PaymentExtensions:              request.PaymentExtensions,
	}
	var sourceTransactionMetadata *entity.SourceTransactionMetadata
	if isPcpTrip || isUpfrontFeatureEnabled {
		sourceTransactionMetadata = &entity.SourceTransactionMetadata{}
		if isPcpTrip {
			sourceTransactionMetadata.IsPcpTrip = &isPcpTrip
		}
		if isUpfrontFeatureEnabled {
			sourceTransactionMetadata.IsUpfrontTransaction = &isUpfrontFeatureEnabled
		}
	}
	if sourceTransactionMetadata != nil {
		capitalBill.SourceTransactionMetadata = sourceTransactionMetadata
	}
	if isUpfrontFeatureEnabled {
		capitalBill.State = entity.CapitalBillStateInProgress
		capitalBill.SourceUberTransactionId = request.UberTransactionID
		logger.Info("UpfrontTransaction - changing capital bill state",
			zap.String("gulfstreamJobType", string(*request.GulfstreamJobType)),
			zap.Bool("isUpfrontTransaction", isUpfrontFeatureEnabled),
			zap.String("capitalBillState", string(capitalBill.State)),
			zap.String("sourceUberTransactionId", safe.String(request.UberTransactionID)),
		)
	}
	createdBill, err := c.capitalbillr.SetBill(ctx, capitalBill)
	if err != nil {
		return nil, err
	}
	if createdBill.InstrumentType == entity.CapitalBillInstrumentTypeUCharge {
		var authHoldInfo *entity.RestrictorCaptureAuthHoldInfo
		if request.JobUUID != nil {
			authHoldInfo = &entity.RestrictorCaptureAuthHoldInfo{
				JobUUID:                  *request.JobUUID,
				UberTransactionID:        request.UberTransactionID,
				PaymentAccountOwnerUUID:  &paymentAccountOwnerUUID,
				PaymentAccountEntityType: entityType,
				HoldAmount:               request.Amount,
				AuthChargeAction:         request.AuthChargeAction,
			}
		}
		if err = c.cadence.EnqueueChargeLedgerForCapitalBillWorkflow(ctx, createdBill.UUID, authHoldInfo); err != nil {
			return nil, err
		}
	} else {
		err = c.cadence.EnqueueChargeCapitalBillWorkflow(ctx, createdBill.UUID)
		if err != nil {
			return nil, err
		}
	}

	// Check how flair is logging originator and actor from office-space headers
	if ((createdBill.Originator != nil) && (*createdBill.Originator == constants.OfficeSpace)) ||
		(createdBill.Type == entity.CapitalBillTypeUChargePayment) {
		// If the capital bill is UCHARGE_PAYMENT, UCHARGE_CREDIT, SERVICE_FEE,  MISC_CHARGE or VOUCHERS_CREATION_FEE, log it in Audit Log
		logActivityRequest := &entity.LogActivityForAuditLogRequest{
			Bill:        createdBill,
			CallSiteTag: "controllers.service-0001.Charge",
		}
		_ = c.logActivityForAuditLog(ctx, logActivityRequest, paymentAccountType)
	}
	if isClearinghouseTrafficVal {
		if paymentAccountInfo != nil &&
			paymentAccountInfo.PaymentAccountEntityType != nil &&
			*paymentAccountInfo.PaymentAccountEntityType == entity.NinaAccountEntityTypeFamilyGroup {
			if config.AppCfg.Configuration.U4FMembersArrears.Enabled ||
				(request.ClientUUID != nil &&
					contains.String(config.AppCfg.Configuration.U4FMembersArrears.Allowlist, request.ClientUUID.String())) {
				if request.ClientUUID != nil && *request.ClientUUID != paymentAccountInfo.PaymentProfileOwnerUUID {
					scope.Counter("charge.family.success").Inc(1)
					state := entity.CapitalBillStateInProgress
					return &entity.ChargeResponse{
						CapitalBillUUID:  ptr.UUID(createdBill.UUID),
						CapitalBillState: &state,
					}, nil
				}
				scope.Counter("charge.family.organizer").Inc(1)
			} else {
				scope.Counter("charge.family.skip").Inc(1)
			}
		}
	}

	// For upfront transactions, return the state as in_progress to PIS
	var state *entity.CapitalBillState
	if isUpfrontFeatureEnabled {
		logger.Info("UpfrontTransaction - sending inProgressState", zap.String("capitalBillState", createdBill.State.String()))
		state = &createdBill.State
	}
	return &entity.ChargeResponse{CapitalBillUUID: ptr.UUID(createdBill.UUID), CapitalBillState: state}, nil
}
