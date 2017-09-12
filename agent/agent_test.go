package agent

import (
	"testing"
	"time"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/apimodels"
	"github.com/evergreen-ci/evergreen/command"
	"github.com/evergreen-ci/evergreen/model"
	"github.com/evergreen-ci/evergreen/model/task"
	"github.com/evergreen-ci/evergreen/rest/client"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
)

type AgentTestSuite struct {
	suite.Suite
	a                Agent
	mockCommunicator *client.Mock
	tc               *taskContext
}

func TestAgentTestSuite(t *testing.T) {
	suite.Run(t, new(AgentTestSuite))
}

func (s *AgentTestSuite) SetupTest() {
	s.a = Agent{
		opts: Options{
			HostID:     "host",
			HostSecret: "secret",
			StatusPort: 2286,
			LogPrefix:  evergreen.LocalLoggingOverride,
		},
		comm: client.NewMock("url"),
	}
	s.mockCommunicator = s.a.comm.(*client.Mock)

	s.tc = &taskContext{
		task: client.TaskData{
			ID:     "task_id",
			Secret: "task_secret",
		},
		taskConfig: &model.TaskConfig{
			Project: &model.Project{},
		},
	}
	s.tc.logger = s.a.comm.GetLoggerProducer(context.Background(), s.tc.task)

	factory, ok := command.GetCommandFactory("setup.initial")
	s.True(ok)
	s.tc.currentCommand = factory()

}

func (s *AgentTestSuite) TestNextTaskResponseShouldExit() {
	s.mockCommunicator.NextTaskResponse = &apimodels.NextTaskResponse{
		TaskId:     "mocktaskid",
		TaskSecret: "",
		ShouldExit: true}
	err := s.a.loop(context.Background())
	s.Error(err)
}

func (s *AgentTestSuite) TestTaskWithoutSecret() {
	s.mockCommunicator.NextTaskResponse = &apimodels.NextTaskResponse{
		TaskId:     "mocktaskid",
		TaskSecret: "",
		ShouldExit: false}
	err := s.a.loop(context.Background())
	s.Error(err)
}

func (s *AgentTestSuite) TestErrorGettingNextTask() {
	s.mockCommunicator.NextTaskShouldFail = true
	err := s.a.loop(context.Background())
	s.Error(err)
}

func (s *AgentTestSuite) TestCanceledContext() {
	s.a.opts.AgentSleepInterval = time.Millisecond
	s.mockCommunicator.NextTaskIsNil = true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := s.a.loop(ctx)
	s.NoError(err)
}

func (s *AgentTestSuite) TestAgentEndTaskShouldExit() {
	s.mockCommunicator.EndTaskResponse = &apimodels.EndTaskResponse{ShouldExit: true}
	err := s.a.loop(context.Background())
	s.Error(err)
}

func (s *AgentTestSuite) TestFinishTaskReturnsEndTaskResponse() {
	endTaskResponse := &apimodels.EndTaskResponse{Message: "end task response"}
	s.mockCommunicator.EndTaskResponse = endTaskResponse
	resp, err := s.a.finishTask(context.Background(), s.tc, evergreen.TaskSucceeded, true)
	s.Equal(endTaskResponse, resp)
	s.NoError(err)
}

func (s *AgentTestSuite) TestFinishTaskEndTaskError() {

	s.mockCommunicator.EndTaskShouldFail = true
	resp, err := s.a.finishTask(context.Background(), s.tc, evergreen.TaskSucceeded, true)
	s.Nil(resp)
	s.Error(err)
}

func (s *AgentTestSuite) TestCancelStartTask() {
	idleTimeout := make(chan time.Duration)
	complete := make(chan string)
	execTimeout := make(chan struct{})
	go func() {
		for _ = range idleTimeout {
			// discard
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.a.startTask(ctx, s.tc, complete, execTimeout, idleTimeout)
	msgs := s.mockCommunicator.GetMockMessages()
	s.Zero(len(msgs))
}

func (s *AgentTestSuite) TestCancelRunCommands() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := model.PluginCommandConf{
		Command: "shell.exec",
		Params: map[string]interface{}{
			"script": "echo hi",
		},
	}
	cmds := []model.PluginCommandConf{cmd}
	idleTimeout := make(chan time.Duration)
	err := s.a.runCommands(ctx, s.tc, cmds, false, idleTimeout)
	s.Error(err)
	s.Equal("runCommands canceled", err.Error())
}

func (s *AgentTestSuite) TestRunPreTaskCommands() {
	s.tc.taskConfig = &model.TaskConfig{
		BuildVariant: &model.BuildVariant{
			Name: "buildvariant_id",
		},
		Task: &task.Task{
			Id: "task_id",
		},
		Project: &model.Project{
			Pre: &model.YAMLCommandSet{
				SingleCommand: &model.PluginCommandConf{
					Command: "shell.exec",
					Params: map[string]interface{}{
						"script": "echo hi",
					},
				},
			},
		},
	}
	s.a.runPreTaskCommands(context.Background(), s.tc)

	_ = s.tc.logger.Close()
	msgs := s.mockCommunicator.GetMockMessages()["task_id"]
	s.Equal("Running pre-task commands.", msgs[0].Message)
	s.Equal("Running command 'shell.exec' (step 1 of 1)", msgs[1].Message)
	s.Contains(msgs[len(msgs)-1].Message, "Finished running pre-task commands")
}

func (s *AgentTestSuite) TestRunPostTaskCommands() {
	s.tc.taskConfig = &model.TaskConfig{
		BuildVariant: &model.BuildVariant{
			Name: "buildvariant_id",
		},
		Task: &task.Task{
			Id: "task_id",
		},
		Project: &model.Project{
			Post: &model.YAMLCommandSet{
				SingleCommand: &model.PluginCommandConf{
					Command: "shell.exec",
					Params: map[string]interface{}{
						"script": "echo hi",
					},
				},
			},
		},
	}
	s.a.runPostTaskCommands(context.Background(), s.tc)
	_ = s.tc.logger.Close()
	msgs := s.mockCommunicator.GetMockMessages()["task_id"]
	s.Equal("Running post-task commands.", msgs[0].Message)
	s.Equal("Running command 'shell.exec' (step 1 of 1)", msgs[1].Message)
	s.Contains(msgs[len(msgs)-1].Message, "Finished running post-task commands")
}

func (s *AgentTestSuite) TestEndTaskResponse() {
	factory, ok := command.GetCommandFactory("setup.initial")
	s.True(ok)
	s.tc.currentCommand = factory()

	detail := s.a.endTaskResponse(s.tc, evergreen.TaskSucceeded, true)
	s.True(detail.TimedOut)
	s.Equal(evergreen.TaskSucceeded, detail.Status)

	detail = s.a.endTaskResponse(s.tc, evergreen.TaskSucceeded, false)
	s.False(detail.TimedOut)
	s.Equal(evergreen.TaskSucceeded, detail.Status)

	detail = s.a.endTaskResponse(s.tc, evergreen.TaskFailed, true)
	s.True(detail.TimedOut)
	s.Equal(evergreen.TaskFailed, detail.Status)

	detail = s.a.endTaskResponse(s.tc, evergreen.TaskFailed, false)
	s.False(detail.TimedOut)
	s.Equal(evergreen.TaskFailed, detail.Status)
}

func (s *AgentTestSuite) TestAbort() {
	s.mockCommunicator.HeartbeatShouldAbort = true
	err := s.a.runTask(context.Background(), s.tc)
	s.NoError(err)
	s.Equal(evergreen.TaskFailed, s.mockCommunicator.EndTaskResult.Detail.Status)
	s.Equal("initial task setup", s.mockCommunicator.EndTaskResult.Detail.Description)
}

func (s *AgentTestSuite) TestAgentConstructorSetsHostData() {
	agent, err := New(Options{HostID: "host_id", HostSecret: "host_secret"}, client.NewMock("url"))
	s.NoError(err)
	s.Equal("host_id", agent.comm.GetHostID())
	s.Equal("host_secret", agent.comm.GetHostSecret())
}

func (s *AgentTestSuite) TestWaitCompleteSuccess() {
	heartbeat := make(chan string)
	idleTimeout := make(chan struct{})
	complete := make(chan string)
	execTimeout := make(chan struct{})
	go func() {
		complete <- evergreen.TaskSucceeded
	}()
	status, timeout := s.a.wait(context.Background(), s.tc, heartbeat, idleTimeout, complete, execTimeout)
	s.Equal(evergreen.TaskSucceeded, status)
	s.False(timeout)
}

func (s *AgentTestSuite) TestWaitCompleteFailure() {
	heartbeat := make(chan string)
	idleTimeout := make(chan struct{})
	complete := make(chan string)
	execTimeout := make(chan struct{})
	go func() {
		complete <- evergreen.TaskFailed
	}()
	status, timeout := s.a.wait(context.Background(), s.tc, heartbeat, idleTimeout, complete, execTimeout)
	s.Equal(evergreen.TaskFailed, status)
	s.False(timeout)
}

func (s *AgentTestSuite) TestWaitExecTimeout() {
	heartbeat := make(chan string)
	idleTimeout := make(chan struct{})
	complete := make(chan string)
	execTimeout := make(chan struct{})
	close(execTimeout)
	status, timeout := s.a.wait(context.Background(), s.tc, heartbeat, idleTimeout, complete, execTimeout)
	s.Equal(evergreen.TaskFailed, status)
	s.False(timeout)
}

func (s *AgentTestSuite) TestWaitHeartbeatTimeout() {
	heartbeat := make(chan string)
	idleTimeout := make(chan struct{})
	complete := make(chan string)
	execTimeout := make(chan struct{})
	go func() {
		heartbeat <- evergreen.TaskUndispatched
	}()
	status, timeout := s.a.wait(context.Background(), s.tc, heartbeat, idleTimeout, complete, execTimeout)
	s.Equal(evergreen.TaskUndispatched, status)
	s.False(timeout)
}

func (s *AgentTestSuite) TestWaitIdleTimeout() {
	s.tc = &taskContext{
		task: client.TaskData{
			ID:     "task_id",
			Secret: "task_secret",
		},
		taskConfig: &model.TaskConfig{
			BuildVariant: &model.BuildVariant{
				Name: "buildvariant_id",
			},
			Task: &task.Task{
				Id: "task_id",
			},
			Project: &model.Project{
				Timeout: &model.YAMLCommandSet{
					SingleCommand: &model.PluginCommandConf{
						Command: "shell.exec",
						Params: map[string]interface{}{
							"script": "echo hi",
						},
					},
				},
			},
		},
	}
	s.tc.logger = s.a.comm.GetLoggerProducer(context.Background(), s.tc.task)
	factory, ok := command.GetCommandFactory("setup.initial")
	s.True(ok)
	s.tc.currentCommand = factory()

	heartbeat := make(chan string)
	idleTimeout := make(chan struct{})
	complete := make(chan string)
	execTimeout := make(chan struct{})
	close(idleTimeout)
	status, timeout := s.a.wait(context.Background(), s.tc, heartbeat, idleTimeout, complete, execTimeout)
	s.Equal(evergreen.TaskFailed, status)
	s.True(timeout)
}
