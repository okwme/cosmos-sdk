package keeper_test

import (
	gocontext "context"
	"fmt"
	"strconv"
	"testing"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/simapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/suite"
	abci "github.com/tendermint/tendermint/abci/types"
)

type GRPCTestSuite struct {
	suite.Suite

	app         *simapp.SimApp
	ctx         sdk.Context
	queryClient types.QueryClient
	addrs       []sdk.AccAddress
}

func (suite *GRPCTestSuite) SetupTest() {
	app := simapp.Setup(false)
	ctx := app.BaseApp.NewContext(false, abci.Header{})

	queryHelper := baseapp.NewQueryServerTestHelper(ctx, app.InterfaceRegistry())
	types.RegisterQueryServer(queryHelper, app.GovKeeper)
	queryClient := types.NewQueryClient(queryHelper)

	suite.app = app
	suite.ctx = ctx
	suite.queryClient = queryClient
	suite.addrs = simapp.AddTestAddrsIncremental(app, ctx, 2, sdk.NewInt(30000000))
}

func (suite *GRPCTestSuite) TestGRPCQueryProposal() {
	app, ctx, queryClient := suite.app, suite.ctx, suite.queryClient

	var (
		req         *types.QueryProposalRequest
		expProposal types.Proposal
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"empty request",
			func() {
				req = &types.QueryProposalRequest{}
			},
			false,
		},
		{
			"non existing proposal request",
			func() {
				req = &types.QueryProposalRequest{ProposalId: 3}
			},
			false,
		},
		{
			"valid request",
			func() {
				req = &types.QueryProposalRequest{ProposalId: 1}
				testProposal := types.NewTextProposal("Proposal", "testing proposal")
				submittedProposal, err := app.GovKeeper.SubmitProposal(ctx, testProposal)
				suite.Require().NoError(err)
				suite.Require().NotEmpty(submittedProposal)

				expProposal = submittedProposal
			},
			true,
		},
	}

	for _, testCase := range testCases {
		suite.Run(fmt.Sprintf("Case %s", testCase.msg), func() {
			testCase.malleate()

			proposalRes, err := queryClient.Proposal(gocontext.Background(), req)

			if testCase.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(expProposal.String(), proposalRes.Proposal.String())
			} else {
				suite.Require().Error(err)
				suite.Require().Nil(proposalRes)
			}
		})
	}
}

func (suite *GRPCTestSuite) TestGRPCQueryProposals() {
	app, ctx, queryClient, addrs := suite.app, suite.ctx, suite.queryClient, suite.addrs

	testProposals := []types.Proposal{}

	var (
		req    *types.QueryProposalsRequest
		expRes *types.QueryProposalsResponse
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"empty state request",
			func() {
				req = &types.QueryProposalsRequest{}
			},
			true,
		},
		{
			"request proposals with limit 3",
			func() {
				// create 5 test proposals
				for i := 0; i < 5; i++ {
					num := strconv.Itoa(i + 1)
					testProposal := types.NewTextProposal("Proposal"+num, "testing proposal "+num)
					proposal, err := app.GovKeeper.SubmitProposal(ctx, testProposal)
					suite.Require().NotEmpty(proposal)
					suite.Require().NoError(err)
					testProposals = append(testProposals, proposal)
				}

				req = &types.QueryProposalsRequest{
					Req: &query.PageRequest{Limit: 3},
				}

				expRes = &types.QueryProposalsResponse{
					Proposals: testProposals[:3],
				}
			},
			true,
		},
		{
			"request 2nd page with limit 4",
			func() {
				req = &types.QueryProposalsRequest{
					Req: &query.PageRequest{Offset: 3, Limit: 3},
				}

				expRes = &types.QueryProposalsResponse{
					Proposals: testProposals[3:],
				}
			},
			true,
		},
		{
			"request with limit 3 and count all",
			func() {
				req = &types.QueryProposalsRequest{
					Req: &query.PageRequest{Limit: 2, CountTotal: true},
				}

				expRes = &types.QueryProposalsResponse{
					Proposals: testProposals[:2],
				}
			},
			true,
		},
		{
			"request with filter of status deposit period",
			func() {
				req = &types.QueryProposalsRequest{
					ProposalStatus: types.StatusDepositPeriod,
				}

				expRes = &types.QueryProposalsResponse{
					Proposals: testProposals,
				}
			},
			true,
		},
		{
			"request with filter of deposit address",
			func() {
				depositCoins := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.TokensFromConsensusPower(20)))
				deposit := types.NewDeposit(testProposals[0].ProposalID, addrs[0], depositCoins)
				app.GovKeeper.SetDeposit(ctx, deposit)

				req = &types.QueryProposalsRequest{
					Depositor: addrs[0],
				}

				expRes = &types.QueryProposalsResponse{
					Proposals: testProposals[:1],
				}
			},
			true,
		},
		{
			"request with filter of deposit address",
			func() {
				testProposals[1].Status = types.StatusVotingPeriod
				app.GovKeeper.SetProposal(ctx, testProposals[1])
				suite.Require().NoError(app.GovKeeper.AddVote(ctx, testProposals[1].ProposalID, addrs[0], types.OptionAbstain))

				req = &types.QueryProposalsRequest{
					Voter: addrs[0],
				}

				expRes = &types.QueryProposalsResponse{
					Proposals: testProposals[1:2],
				}
			},
			true,
		},
	}

	for _, testCase := range testCases {
		suite.Run(fmt.Sprintf("Case %s", testCase.msg), func() {
			testCase.malleate()

			proposals, err := queryClient.Proposals(gocontext.Background(), req)

			if testCase.expPass {
				suite.Require().NoError(err)

				suite.Require().Len(proposals.GetProposals(), len(expRes.GetProposals()))
				for i := 0; i < len(proposals.GetProposals()); i++ {
					suite.Require().Equal(proposals.GetProposals()[i].String(), expRes.GetProposals()[i].String())
				}

			} else {
				suite.Require().Error(err)
				suite.Require().Nil(proposals)
			}
		})
	}
}

func (suite *GRPCTestSuite) TestGRPCQueryVote() {
	app, ctx, queryClient, addrs := suite.app, suite.ctx, suite.queryClient, suite.addrs

	var (
		req      *types.QueryVoteRequest
		expRes   *types.QueryVoteResponse
		proposal types.Proposal
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"empty request",
			func() {
				req = &types.QueryVoteRequest{}
			},
			false,
		},
		{
			"non existed proposal",
			func() {
				req = &types.QueryVoteRequest{
					ProposalId: 3,
					Voter:      addrs[0],
				}
			},
			false,
		},
		{
			"no votes present",
			func() {
				var err error
				proposal, err = app.GovKeeper.SubmitProposal(ctx, TestProposal)
				suite.Require().NoError(err)

				req = &types.QueryVoteRequest{
					ProposalId: proposal.ProposalID,
					Voter:      addrs[0],
				}

				expRes = &types.QueryVoteResponse{}
			},
			false,
		},
		{
			"valid request",
			func() {
				proposal.Status = types.StatusVotingPeriod
				app.GovKeeper.SetProposal(ctx, proposal)
				suite.Require().NoError(app.GovKeeper.AddVote(ctx, proposal.ProposalID, addrs[0], types.OptionAbstain))

				req = &types.QueryVoteRequest{
					ProposalId: proposal.ProposalID,
					Voter:      addrs[0],
				}

				expRes = &types.QueryVoteResponse{Vote: types.NewVote(proposal.ProposalID, addrs[0], types.OptionAbstain)}
			},
			true,
		},
		{
			"wrong voter id request",
			func() {
				req = &types.QueryVoteRequest{
					ProposalId: proposal.ProposalID,
					Voter:      addrs[1],
				}

				expRes = &types.QueryVoteResponse{}
			},
			false,
		},
	}

	for _, testCase := range testCases {
		suite.Run(fmt.Sprintf("Case %s", testCase.msg), func() {
			testCase.malleate()

			vote, err := queryClient.Vote(gocontext.Background(), req)

			if testCase.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(expRes, vote)
			} else {
				suite.Require().Error(err)
				suite.Require().Nil(vote)
			}
		})
	}
}

func (suite *GRPCTestSuite) TestGRPCQueryVotes() {
	app, ctx, queryClient := suite.app, suite.ctx, suite.queryClient

	addrs := simapp.AddTestAddrsIncremental(app, ctx, 2, sdk.NewInt(30000000))

	var (
		req      *types.QueryVotesRequest
		expRes   *types.QueryVotesResponse
		proposal types.Proposal
		votes    types.Votes
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"empty request",
			func() {
				req = &types.QueryVotesRequest{}
			},
			false,
		},
		{
			"non existed proposals",
			func() {
				req = &types.QueryVotesRequest{
					ProposalId: 2,
				}
			},
			true,
		},
		{
			"create a proposal and get votes",
			func() {
				var err error
				proposal, err = app.GovKeeper.SubmitProposal(ctx, TestProposal)
				suite.Require().NoError(err)

				req = &types.QueryVotesRequest{
					ProposalId: proposal.ProposalID,
				}
			},
			true,
		},
		{
			"request after adding 2 votes",
			func() {
				proposal.Status = types.StatusVotingPeriod
				app.GovKeeper.SetProposal(ctx, proposal)

				votes = []types.Vote{
					{proposal.ProposalID, addrs[0], types.OptionAbstain},
					{proposal.ProposalID, addrs[1], types.OptionYes},
				}

				suite.Require().NoError(app.GovKeeper.AddVote(ctx, proposal.ProposalID, votes[0].Voter, votes[0].Option))
				suite.Require().NoError(app.GovKeeper.AddVote(ctx, proposal.ProposalID, votes[1].Voter, votes[1].Option))

				req = &types.QueryVotesRequest{
					ProposalId: proposal.ProposalID,
				}

				expRes = &types.QueryVotesResponse{
					Votes: votes,
				}
			},
			true,
		},
	}

	for _, testCase := range testCases {
		suite.Run(fmt.Sprintf("Case %s", testCase.msg), func() {
			testCase.malleate()

			votes, err := queryClient.Votes(gocontext.Background(), req)

			if testCase.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(expRes.GetVotes(), votes.GetVotes())
			} else {
				suite.Require().Error(err)
				suite.Require().Nil(votes)
			}
		})
	}
}

func (suite *GRPCTestSuite) TestGRPCQueryParams() {
	queryClient := suite.queryClient

	var (
		req    *types.QueryParamsRequest
		expRes *types.QueryParamsResponse
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"empty request",
			func() {
				req = &types.QueryParamsRequest{}
			},
			false,
		},
		{
			"deposit params requset",
			func() {
				req = &types.QueryParamsRequest{ParamsType: types.ParamDeposit}
				expRes = &types.QueryParamsResponse{
					DepositParams: types.DefaultDepositParams(),
					TallyParams:   types.NewTallyParams(sdk.NewDec(0), sdk.NewDec(0), sdk.NewDec(0)),
				}
			},
			true,
		},
		{
			"voting params requset",
			func() {
				req = &types.QueryParamsRequest{ParamsType: types.ParamVoting}
				expRes = &types.QueryParamsResponse{
					VotingParams: types.DefaultVotingParams(),
					TallyParams:  types.NewTallyParams(sdk.NewDec(0), sdk.NewDec(0), sdk.NewDec(0)),
				}
			},
			true,
		},
		{
			"tally params requset",
			func() {
				req = &types.QueryParamsRequest{ParamsType: types.ParamTallying}
				expRes = &types.QueryParamsResponse{
					TallyParams: types.DefaultTallyParams(),
				}
			},
			true,
		},
		{
			"tally params requset",
			func() {
				req = &types.QueryParamsRequest{ParamsType: "wrongPath"}
				expRes = &types.QueryParamsResponse{}
			},
			false,
		},
	}

	for _, testCase := range testCases {
		suite.Run(fmt.Sprintf("Case %s", testCase.msg), func() {
			testCase.malleate()

			params, err := queryClient.Params(gocontext.Background(), req)

			if testCase.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(expRes.GetDepositParams(), params.GetDepositParams())
				suite.Require().Equal(expRes.GetVotingParams(), params.GetVotingParams())
				suite.Require().Equal(expRes.GetTallyParams(), params.GetTallyParams())
			} else {
				suite.Require().Error(err)
				suite.Require().Nil(params)
			}
		})
	}
}

func (suite *GRPCTestSuite) TestGRPCQueryDeposit() {
	app, ctx, queryClient, addrs := suite.app, suite.ctx, suite.queryClient, suite.addrs

	var (
		req      *types.QueryDepositRequest
		expRes   *types.QueryDepositResponse
		proposal types.Proposal
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"empty request",
			func() {
				req = &types.QueryDepositRequest{}
			},
			false,
		},
		{
			"non existed proposal",
			func() {
				req = &types.QueryDepositRequest{
					ProposalId: 2,
					Depositor:  addrs[0],
				}
			},
			false,
		},
		{
			"no deposits proposal",
			func() {
				var err error
				proposal, err = app.GovKeeper.SubmitProposal(ctx, TestProposal)
				suite.Require().NoError(err)
				suite.Require().NotNil(proposal)

				req = &types.QueryDepositRequest{
					ProposalId: proposal.ProposalID,
					Depositor:  addrs[0],
				}
			},
			false,
		},
		{
			"valid request",
			func() {
				depositCoins := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.TokensFromConsensusPower(20)))
				deposit := types.NewDeposit(proposal.ProposalID, addrs[0], depositCoins)
				app.GovKeeper.SetDeposit(ctx, deposit)

				req = &types.QueryDepositRequest{
					ProposalId: proposal.ProposalID,
					Depositor:  addrs[0],
				}

				expRes = &types.QueryDepositResponse{Deposit: deposit}
			},
			true,
		},
	}

	for _, testCase := range testCases {
		suite.Run(fmt.Sprintf("Case %s", testCase.msg), func() {
			testCase.malleate()

			deposit, err := queryClient.Deposit(gocontext.Background(), req)

			if testCase.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(deposit.GetDeposit(), expRes.GetDeposit())
			} else {
				suite.Require().Error(err)
				suite.Require().Nil(expRes)
			}
		})
	}
}

func (suite *GRPCTestSuite) TestGRPCQueryDeposits() {
	app, ctx, queryClient, addrs := suite.app, suite.ctx, suite.queryClient, suite.addrs

	var (
		req      *types.QueryDepositsRequest
		expRes   *types.QueryDepositsResponse
		proposal types.Proposal
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"empty request",
			func() {
				req = &types.QueryDepositsRequest{}
			},
			false,
		},
		{
			"non existed proposal",
			func() {
				req = &types.QueryDepositsRequest{
					ProposalId: 2,
				}
			},
			true,
		},
		{
			"create a proposal and get deposits",
			func() {
				var err error
				proposal, err = app.GovKeeper.SubmitProposal(ctx, TestProposal)
				suite.Require().NoError(err)

				req = &types.QueryDepositsRequest{
					ProposalId: proposal.ProposalID,
				}
			},
			true,
		},
		{
			"get deposits with default limit",
			func() {
				depositAmount1 := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.TokensFromConsensusPower(20)))
				deposit1 := types.NewDeposit(proposal.ProposalID, addrs[0], depositAmount1)
				app.GovKeeper.SetDeposit(ctx, deposit1)

				depositAmount2 := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.TokensFromConsensusPower(30)))
				deposit2 := types.NewDeposit(proposal.ProposalID, addrs[1], depositAmount2)
				app.GovKeeper.SetDeposit(ctx, deposit2)

				deposits := types.Deposits{deposit1, deposit2}

				req = &types.QueryDepositsRequest{
					ProposalId: proposal.ProposalID,
				}

				expRes = &types.QueryDepositsResponse{
					Deposits: deposits,
				}
			},
			true,
		},
	}

	for _, testCase := range testCases {
		suite.Run(fmt.Sprintf("Case %s", testCase.msg), func() {
			testCase.malleate()

			deposits, err := queryClient.Deposits(gocontext.Background(), req)

			if testCase.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(expRes.GetDeposits(), deposits.GetDeposits())
			} else {
				suite.Require().Error(err)
				suite.Require().Nil(deposits)
			}
		})
	}

	// pageReq := &query.PageRequest{Limit: 1}

	// req := &types.QueryDepositsRequest{
	// 	ProposalId: 0,
	// 	Req:        pageReq,
	// }

	// deposits, err := queryClient.Deposits(gocontext.Background(), req)
	// suite.Require().Error(err)
	// suite.Require().Nil(deposits)

	// proposal, err := app.GovKeeper.SubmitProposal(ctx, TestProposal)
	// suite.Require().NoError(err)
	// proposalID := proposal.ProposalID

	// req = &types.QueryDepositsRequest{
	// 	ProposalId: proposalID,
	// 	Req:        pageReq,
	// }

	// deposits, err = queryClient.Deposits(gocontext.Background(), req)
	// suite.Require().NoError(err)
	// suite.Require().Empty(deposits.Deposits)
	// suite.Require().Empty(deposits.Res)

	// depositAmount1 := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.TokensFromConsensusPower(20)))
	// deposit1 := types.NewDeposit(proposalID, addrs[0], depositAmount1)
	// app.GovKeeper.SetDeposit(ctx, deposit1)

	// depositAmount2 := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.TokensFromConsensusPower(30)))
	// deposit2 := types.NewDeposit(proposalID, addrs[1], depositAmount2)
	// app.GovKeeper.SetDeposit(ctx, deposit2)

	// proposal.Status = types.StatusVotingPeriod
	// app.GovKeeper.SetProposal(ctx, proposal)

	// pageReq = &query.PageRequest{Limit: 1}

	// req = &types.QueryDepositsRequest{
	// 	ProposalId: proposalID,
	// 	Req:        pageReq,
	// }

	// deposits, err = queryClient.Deposits(gocontext.Background(), req)

	// suite.Require().NoError(err)
	// suite.Require().Len(deposits.Deposits, 1)
	// suite.Require().Equal(depositAmount1, deposits.Deposits[0].Amount)
	// suite.Require().NotEmpty(deposits.Res.NextKey)

	// // query vote with limit 1, next key and expect NextKey to be nil.
	// pageReq = &query.PageRequest{
	// 	Key:   deposits.Res.NextKey,
	// 	Limit: 1,
	// }

	// req = &types.QueryDepositsRequest{
	// 	ProposalId: proposalID,
	// 	Req:        pageReq,
	// }

	// deposits, err = queryClient.Deposits(gocontext.Background(), req)
	// suite.Require().NoError(err)
	// suite.Require().Len(deposits.Deposits, 1)
	// suite.Require().Equal(depositAmount2, deposits.Deposits[0].Amount)
	// suite.Require().Empty(deposits.Res)

	// // query vote with limit 2 and expect NextKey to be nil.
	// pageReq = &query.PageRequest{Limit: 2}

	// req = &types.QueryDepositsRequest{
	// 	ProposalId: proposalID,
	// 	Req:        pageReq,
	// }

	// deposits, err = queryClient.Deposits(gocontext.Background(), req)
	// suite.Require().NoError(err)
	// suite.Require().Len(deposits.Deposits, 2)
	// suite.Require().Empty(deposits.Res)
}

func (suite *GRPCTestSuite) TestGRPCQueryTally() {
	app, ctx, queryClient := suite.app, suite.ctx, suite.queryClient

	addrs, _ := createValidators(ctx, app, []int64{5, 5, 5})

	var (
		req      *types.QueryTallyResultRequest
		expRes   *types.QueryTallyResultResponse
		proposal types.Proposal
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"empty request",
			func() {
				req = &types.QueryTallyResultRequest{}
			},
			false,
		},
		{
			"query non existed proposal",
			func() {
				req = &types.QueryTallyResultRequest{ProposalId: 1}
			},
			false,
		},
		{
			"create a proposal and get tally",
			func() {
				var err error
				proposal, err = app.GovKeeper.SubmitProposal(ctx, TestProposal)
				suite.Require().NoError(err)
				suite.Require().NotNil(proposal)

				req = &types.QueryTallyResultRequest{ProposalId: proposal.ProposalID}

				expRes = &types.QueryTallyResultResponse{
					Tally: types.EmptyTallyResult(),
				}
			},
			true,
		},
		{
			"request tally after few votes",
			func() {
				proposal.Status = types.StatusVotingPeriod
				app.GovKeeper.SetProposal(ctx, proposal)

				suite.Require().NoError(app.GovKeeper.AddVote(ctx, proposal.ProposalID, addrs[0], types.OptionYes))
				suite.Require().NoError(app.GovKeeper.AddVote(ctx, proposal.ProposalID, addrs[1], types.OptionYes))
				suite.Require().NoError(app.GovKeeper.AddVote(ctx, proposal.ProposalID, addrs[2], types.OptionYes))

				req = &types.QueryTallyResultRequest{ProposalId: proposal.ProposalID}

				expRes = &types.QueryTallyResultResponse{
					Tally: types.TallyResult{
						Yes: sdk.NewInt(3 * 5 * 1000000),
					},
				}
			},
			true,
		},
		{
			"request final tally after status changed",
			func() {
				proposal.Status = types.StatusPassed
				app.GovKeeper.SetProposal(ctx, proposal)
				proposal, _ = app.GovKeeper.GetProposal(ctx, proposal.ProposalID)

				req = &types.QueryTallyResultRequest{ProposalId: proposal.ProposalID}

				expRes = &types.QueryTallyResultResponse{
					Tally: proposal.FinalTallyResult,
				}
			},
			true,
		},
	}

	for _, testCase := range testCases {
		suite.Run(fmt.Sprintf("Case %s", testCase.msg), func() {
			testCase.malleate()

			tally, err := queryClient.TallyResult(gocontext.Background(), req)

			if testCase.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(expRes.String(), tally.String())
			} else {
				suite.Require().Error(err)
				suite.Require().Nil(tally)
			}
		})
	}
}

func TestGRPCTestSuite(t *testing.T) {
	suite.Run(t, new(GRPCTestSuite))
}