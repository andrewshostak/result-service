update-mocks:
	# alias
	mockery --name=AliasRepository --dir internal/app/alias --output internal/app/alias/mocks --case snake
	mockery --name=ExternalAPIClient --dir internal/app/alias --output internal/app/alias/mocks --case snake
	mockery --name=Logger --dir internal/app/alias --output internal/app/alias/mocks --case snake
    # match
	mockery --name=AliasRepository --dir internal/app/match --output internal/app/match/mocks --case snake
	mockery --name=MatchRepository --dir internal/app/match --output internal/app/match/mocks --case snake
	mockery --name=ExternalMatchRepository --dir internal/app/match --output internal/app/match/mocks --case snake
	mockery --name=CheckResultTaskRepository --dir internal/app/match --output internal/app/match/mocks --case snake
	mockery --name=ExternalAPIClient --dir internal/app/match --output internal/app/match/mocks --case snake
	mockery --name=SubscriptionRepository --dir internal/app/match --output internal/app/match/mocks --case snake
	mockery --name=TaskClient --dir internal/app/match --output internal/app/match/mocks --case snake
	mockery --name=Logger --dir internal/app/match --output internal/app/match/mocks --case snake
	mockery --name=HTTPManager --dir internal/adapters/http/client/fotmob --output internal/adapters/http/client/fotmob/mocks --case snake
	# subscription
	mockery --name=AliasRepository --dir internal/app/subscription --output internal/app/subscription/mocks --case snake
	mockery --name=NotifierClient --dir internal/app/subscription --output internal/app/subscription/mocks --case snake
	mockery --name=MatchRepository --dir internal/app/subscription --output internal/app/subscription/mocks --case snake
	mockery --name=SubscriptionRepository --dir internal/app/subscription --output internal/app/subscription/mocks --case snake
	mockery --name=TaskClient --dir internal/app/subscription --output internal/app/subscription/mocks --case snake
	mockery --name=Logger --dir internal/app/subscription --output internal/app/subscription/mocks --case snake

.PHONY: update-mocks