import type { ScriptJob } from "../types";

async function responseError(response: Response): Promise<Error> {
    const message = (await response.text()).trim();
    return new Error(message || `Request failed with status ${response.status}`);
}

export async function fetchSessionScripts(
    sessionId: string,
): Promise<ScriptJob[]> {
    const response = await fetch(
        `/session/${encodeURIComponent(sessionId)}/jobs`,
    );
    if (!response.ok) {
        throw await responseError(response);
    }
    return (await response.json()) as ScriptJob[];
}

export async function fetchRandomScript(): Promise<ScriptJob | null> {
    const response = await fetch("/scripts/random");
    if (response.status === 404) {
        return null;
    }
    if (!response.ok) {
        throw await responseError(response);
    }
    return (await response.json()) as ScriptJob;
}
