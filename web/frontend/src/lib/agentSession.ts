const SESSION_BOOTSTRAP_ELEMENT_ID = "go-js-session";

export type SessionBootstrap = {
    session_id: string;
    ws_path: string;
    mcp_path: string;
};

export function readSessionBootstrap(): SessionBootstrap | null {
    const element = document.getElementById(SESSION_BOOTSTRAP_ELEMENT_ID);
    if (element === null || element.textContent === null) {
        return null;
    }

    try {
        const data = JSON.parse(element.textContent) as Partial<SessionBootstrap>;
        if (
            typeof data.session_id !== "string" ||
            typeof data.ws_path !== "string" ||
            typeof data.mcp_path !== "string"
        ) {
            return null;
        }

        return {
            session_id: data.session_id,
            ws_path: data.ws_path,
            mcp_path: data.mcp_path,
        };
    } catch {
        return null;
    }
}

export function resolveUrl(path: string): string {
    return new URL(path, window.location.origin).toString();
}
