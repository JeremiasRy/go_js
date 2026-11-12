import { createClientId } from "./clientIdentity";

export type SessionBootstrap = {
    session_id: string;
    ws_path: string;
    mcp_path: string;
};

export function createSessionBootstrap(): SessionBootstrap | null {
    const sessionID = createClientId();
    if (sessionID === null) {
        return null;
    }

    return {
        session_id: sessionID,
        ws_path: `/ws/${sessionID}`,
        mcp_path: `/session/${sessionID}/mcp`,
    };
}

export function resolveUrl(path: string): string {
    return new URL(path, window.location.origin).toString();
}
