package controller

import (
	"encoding/json"
	"go_js_demo/db"
	"go_js_demo/worker"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type interpretRequestBody struct {
	Src string `json:"src"`
}

type interpretResponseBody struct {
	JobId string `string:"job_id"`
}

type userScriptJobResponse struct {
	JobId     uuid.UUID `json:"job_id"`
	Src       string    `json:"src"`
	JobStatus string    `json:"job_status"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserScriptsController struct {
	db  *db.Db
	sig chan worker.NewQuerySignal
}

func userScriptJobFromDbToResponse(userScriptJob *db.UserScriptJob) *userScriptJobResponse {
	var response userScriptJobResponse
	response.JobId = userScriptJob.JobId
	response.Src = userScriptJob.Src
	response.JobStatus = userScriptJob.JobStatus.String()
	response.Result = userScriptJob.Result
	response.CreatedAt = userScriptJob.CreatedAt
	response.UpdatedAt = userScriptJob.UpdatedAt

	return &response
}

func NewUserSriptsController(db *db.Db, sig chan worker.NewQuerySignal) *UserScriptsController {
	return &UserScriptsController{db, sig}
}

func (ctr *UserScriptsController) HandlePostInterpret(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	var requestBody interpretRequestBody
	err := json.NewDecoder(r.Body).Decode(&requestBody)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jobId, err := ctr.db.InsertNewJob(requestBody.Src, r.Context())

	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	ctr.sig <- worker.NewQuerySignal{}
	response, err := json.Marshal(interpretResponseBody{JobId: jobId})

	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func (ctr *UserScriptsController) HandleGetJobById(w http.ResponseWriter, r *http.Request) {
	jobId := r.PathValue("job_id")
	jobIdUuid, err := uuid.Parse(jobId)

	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	job, err := ctr.db.GetJobByJobId(jobIdUuid, r.Context())

	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	response, err := json.Marshal(userScriptJobFromDbToResponse(job))

	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}
