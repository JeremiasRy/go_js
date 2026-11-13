const CLIENT_ID_STORAGE_KEY = "go-js-client-id-v1";
const CLIENT_ID_PATTERN =
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function hash128(input: string): [number, number, number, number] {
    let h1 = 1779033703;
    let h2 = 3144134277;
    let h3 = 1013904242;
    let h4 = 2773480762;

    for (let index = 0; index < input.length; index += 1) {
        const code = input.charCodeAt(index);
        h1 = h2 ^ Math.imul(h1 ^ code, 597399067);
        h2 = h3 ^ Math.imul(h2 ^ code, 2869860233);
        h3 = h4 ^ Math.imul(h3 ^ code, 951274213);
        h4 = h1 ^ Math.imul(h4 ^ code, 2716044179);
    }

    h1 = Math.imul(h3 ^ (h1 >>> 18), 597399067);
    h2 = Math.imul(h4 ^ (h2 >>> 22), 2869860233);
    h3 = Math.imul(h1 ^ (h3 >>> 17), 951274213);
    h4 = Math.imul(h2 ^ (h4 >>> 19), 2716044179);
    h1 ^= h2 ^ h3 ^ h4;
    h2 ^= h1;
    h3 ^= h1;
    h4 ^= h1;

    return [h1 >>> 0, h2 >>> 0, h3 >>> 0, h4 >>> 0];
}

function formatClientID(fingerprint: string): string {
    const hex = hash128(fingerprint)
        .map((part) => part.toString(16).padStart(8, "0"))
        .join("")
        .split("");

    hex[12] = "5";
    hex[16] = ((Number.parseInt(hex[16], 16) & 0x3) | 0x8).toString(16);

    return [
        hex.slice(0, 8).join(""),
        hex.slice(8, 12).join(""),
        hex.slice(12, 16).join(""),
        hex.slice(16, 20).join(""),
        hex.slice(20, 32).join(""),
    ].join("-");
}

function readStoredClientId(): string | null {
    try {
        const stored = window.localStorage.getItem(CLIENT_ID_STORAGE_KEY);
        if (stored !== null && CLIENT_ID_PATTERN.test(stored)) {
            return stored;
        }
    } catch {
        return null;
    }
    return null;
}

function storeClientId(clientId: string) {
    try {
        window.localStorage.setItem(CLIENT_ID_STORAGE_KEY, clientId);
    } catch {
        return;
    }
}

function randomClientId(): string {
    if (typeof crypto.randomUUID === "function") {
        return crypto.randomUUID();
    }
    return formatClientID(`${Date.now()}:${Math.random()}`);
}

async function fingerprintClientId(): Promise<string> {
    const { default: FingerprintJS } = await import(
        "@fingerprintjs/fingerprintjs"
    );
    const agent = await FingerprintJS.load({ monitoring: false });
    const result = await agent.get();
    return clientIdFromFingerprint(result.visitorId);
}

export function clientIdFromFingerprint(fingerprint: string): string {
    return formatClientID(fingerprint);
}

export async function createClientId(): Promise<string | null> {
    if (
        typeof window === "undefined" ||
        typeof document === "undefined" ||
        typeof navigator === "undefined"
    ) {
        return null;
    }

    const stored = readStoredClientId();
    if (stored !== null) {
        return stored;
    }

    let clientId: string;
    try {
        clientId = await fingerprintClientId();
    } catch (error) {
        console.warn("Could not fingerprint this browser", error);
        clientId = randomClientId();
    }

    storeClientId(clientId);
    return clientId;
}
