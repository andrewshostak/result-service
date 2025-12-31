update-mocks:
	mockery --name=AliasRepository --dir service --output service/mocks --case snake
	mockery --name=CheckResultTaskRepository --dir service --output service/mocks --case snake
	mockery --name=FotmobClient --dir service --output service/mocks --case snake
	mockery --name=NotifierClient --dir service --output service/mocks --case snake
	mockery --name=ExternalMatchRepository --dir service --output service/mocks --case snake
	mockery --name=Logger --dir service --output service/mocks --case snake
	mockery --name=MatchRepository --dir service --output service/mocks --case snake
	mockery --name=SubscriptionRepository --dir service --output service/mocks --case snake
	mockery --name=TaskClient --dir service --output service/mocks --case snake

.PHONY: update-mocks