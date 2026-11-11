export type ActiveRunInfo = {
    request_id: string;
    job_id?: string;
};

export type MCPStatus = "waiting" | "active" | "idle";

export type ServerMessage =
    | {
          type: "session.ready";
          session_id: string;
          mcp_status: MCPStatus;
          mcp_last_seen?: string;
          active_run?: ActiveRunInfo;
      }
    | {
          type: "mcp.status";
          mcp_status: MCPStatus;
          mcp_last_seen?: string;
      }
    | {
          type: "execute.request";
          request_id: string;
          code: string;
      }
    | {
          type: "run.accepted";
          request_id: string;
          job_id: string;
      }
    | {
          type: "run.result";
          request_id: string;
          job_id: string;
          job_status: "Success" | "Failed" | "Pending" | "Processing";
          result: string;
      }
    | {
          type: "run.error";
          request_id: string;
          error: string;
      };

export type ClientMessage =
    | {
          type: "run.submit";
          request_id?: string;
          code: string;
      }
    | {
          type: "execute.rejected";
          request_id: string;
          error: string;
      };

export type ConnectionStatus = "connecting" | "connected" | "disconnected";

type AgentSocketOptions = {
    websocketPath: string;
    onMessage: (message: ServerMessage) => void;
    onStatus: (status: ConnectionStatus) => void;
};

export type AgentSocket = {
    send: (message: ClientMessage) => boolean;
    close: () => void;
};

const RECONNECT_BASE_DELAY = 500;
const RECONNECT_MAX_DELAY = 8_000;

export function connectAgentSocket({
    websocketPath,
    onMessage,
    onStatus,
}: AgentSocketOptions): AgentSocket {
    let socket: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let reconnectAttempt = 0;
    let closed = false;

    const websocketUrl = () => {
        const url = new URL(websocketPath, window.location.origin);
        url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
        return url.toString();
    };

    const scheduleReconnect = () => {
        if (closed || reconnectTimer !== null) {
            return;
        }

        const delay = Math.min(
            RECONNECT_MAX_DELAY,
            RECONNECT_BASE_DELAY * 2 ** reconnectAttempt,
        );
        reconnectAttempt += 1;
        reconnectTimer = setTimeout(() => {
            reconnectTimer = null;
            connect();
        }, delay);
    };

    const connect = () => {
        if (closed) {
            return;
        }

        onStatus("connecting");
        const current = new WebSocket(websocketUrl());
        socket = current;

        current.addEventListener("open", () => {
            if (socket !== current) {
                return;
            }
            reconnectAttempt = 0;
            onStatus("connected");
        });

        current.addEventListener("message", (event) => {
            if (socket !== current || typeof event.data !== "string") {
                return;
            }

            try {
                const message = JSON.parse(event.data) as ServerMessage;
                if (typeof message?.type === "string") {
                    onMessage(message);
                }
            } catch {
                console.warn("Ignoring malformed agent websocket message");
            }
        });

        current.addEventListener("close", () => {
            if (socket !== current) {
                return;
            }
            socket = null;
            onStatus("disconnected");
            scheduleReconnect();
        });

        current.addEventListener("error", () => {
            current.close();
        });
    };

    connect();

    return {
        send(message) {
            if (socket === null || socket.readyState !== WebSocket.OPEN) {
                return false;
            }

            try {
                socket.send(JSON.stringify(message));
                return true;
            } catch {
                return false;
            }
        },
        close() {
            closed = true;
            if (reconnectTimer !== null) {
                clearTimeout(reconnectTimer);
                reconnectTimer = null;
            }
            if (socket !== null) {
                socket.close();
                socket = null;
            }
            onStatus("disconnected");
        },
    };
}
