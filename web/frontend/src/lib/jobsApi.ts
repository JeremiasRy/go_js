import type { JobStatus } from "../types";

export type { JobStatus };

export type JobStatusResponse = {
    job_id: string;
    request_id?: string;
    client_request_id?: string;
    job_status: JobStatus;
    version: number;
    created?: boolean;
    status_url?: string;
    result_url?: string | null;
    error?: { code: string; message: string } | null;
    created_at?: string;
    updated_at?: string;
    completed_at?: string | null;
};

export class JobApiError extends Error {
    status: number;
    code?: string;

    constructor(message: string, status: number, code?: string) {
        super(message);
        this.name = "JobApiError";
        this.status = status;
        this.code = code;
    }
}

export class JobPendingError extends JobApiError {
    retryAfterMs: number;

    constructor(message: string, retryAfterMs: number) {
        super(message, 202, "job_pending");
        this.name = "JobPendingError";
        this.retryAfterMs = retryAfterMs;
    }
}

async function responseError(response: Response): Promise<JobApiError> {
    const text = (await response.text()).trim();
    if (text !== "") {
        try {
            const parsed = JSON.parse(text) as {
                error?: { code?: string; message?: string };
            };
            if (parsed.error) {
                return new JobApiError(
                    parsed.error.message || text,
                    response.status,
                    parsed.error.code,
                );
            }
        } catch {
            return new JobApiError(text, response.status);
        }
    }
    return new JobApiError(
        text || `Request failed with status ${response.status}`,
        response.status,
    );
}

function retryAfterMs(response: Response): number {
    const header = response.headers.get("Retry-After");
    if (header === null) {
        return 250;
    }
    const seconds = Number(header);
    if (Number.isFinite(seconds) && seconds >= 0) {
        return seconds * 1_000;
    }
    const date = Date.parse(header);
    if (Number.isNaN(date)) {
        return 250;
    }
    return Math.max(0, date - Date.now());
}

export async function submitJob(
    sessionId: string,
    request: {
        code: string;
        client_request_id: string;
        execute_request_id?: string;
    },
): Promise<JobStatusResponse> {
    const response = await fetch(
        `/session/${encodeURIComponent(sessionId)}/jobs`,
        {
            method: "POST",
            headers: {
                Accept: "application/json",
                "Content-Type": "application/json",
            },
            body: JSON.stringify(request),
        },
    );
    if (!response.ok) {
        throw await responseError(response);
    }
    return (await response.json()) as JobStatusResponse;
}

export async function fetchJobStatus(
    sessionId: string,
    jobId: string,
): Promise<JobStatusResponse> {
    const response = await fetch(
        `/session/${encodeURIComponent(sessionId)}/jobs/${encodeURIComponent(jobId)}`,
        { headers: { Accept: "application/json" } },
    );
    if (!response.ok) {
        throw await responseError(response);
    }
    return (await response.json()) as JobStatusResponse;
}

export async function fetchJobResult(resultUrl: string): Promise<unknown> {
    const response = await fetch(resultUrl, {
        headers: { Accept: "application/json" },
    });
    if (response.status === 202) {
        throw new JobPendingError(
            "The result is not ready yet.",
            retryAfterMs(response),
        );
    }
    if (!response.ok) {
        throw await responseError(response);
    }
    return await response.json();
}
