const IDENTITY_VERSION = "go-js-client-v1";

const FONT_CANDIDATES = [
    "Arial",
    "Arial Black",
    "Calibri",
    "Cambria",
    "Candara",
    "Comic Sans MS",
    "Consolas",
    "Courier New",
    "DejaVu Sans",
    "Georgia",
    "Helvetica Neue",
    "Liberation Sans",
    "Menlo",
    "Monaco",
    "Noto Sans",
    "Roboto",
    "Segoe UI",
    "Tahoma",
    "Times New Roman",
    "Trebuchet MS",
    "Ubuntu",
    "Verdana",
];

type NavigatorWithHints = Navigator & {
    userAgentData?: {
        brands?: Array<{ brand: string; version: string }>;
        mobile?: boolean;
        platform?: string;
    };
    deviceMemory?: number;
    oscpu?: string;
};

function safeValue(read: () => unknown): string {
    try {
        const value = read();
        if (value === undefined || value === null) {
            return "";
        }
        if (Array.isArray(value)) {
            return value.map((item) => String(item)).join(",");
        }
        return String(value);
    } catch {
        return "";
    }
}

function canvasFingerprint(): string {
    return safeValue(() => {
        const canvas = document.createElement("canvas");
        canvas.width = 256;
        canvas.height = 64;

        const context = canvas.getContext("2d");
        if (context === null) {
            return "";
        }

        context.textBaseline = "top";
        context.font = "14px 'Arial'";
        context.fillStyle = "#f60";
        context.fillRect(120, 1, 60, 20);
        context.fillStyle = "#069";
        context.fillText("go_js 世界 🦎 123", 2, 15);
        context.fillStyle = "rgba(102, 204, 0, 0.7)";
        context.fillText("go_js 世界 🦎 123", 4, 17);

        return canvas.toDataURL();
    });
}

function webGLFingerprint(): string {
    return safeValue(() => {
        const canvas = document.createElement("canvas");
        const gl = canvas.getContext("webgl") as WebGLRenderingContext | null;
        if (gl === null) {
            return "";
        }

        const debugInfo = gl.getExtension("WEBGL_debug_renderer_info");
        const vendor = debugInfo
            ? gl.getParameter(debugInfo.UNMASKED_VENDOR_WEBGL)
            : gl.getParameter(gl.VENDOR);
        const renderer = debugInfo
            ? gl.getParameter(debugInfo.UNMASKED_RENDERER_WEBGL)
            : gl.getParameter(gl.RENDERER);

        return [
            vendor,
            renderer,
            gl.getParameter(gl.VERSION),
            gl.getParameter(gl.SHADING_LANGUAGE_VERSION),
            gl.getParameter(gl.MAX_TEXTURE_SIZE),
            gl.getParameter(gl.MAX_RENDERBUFFER_SIZE),
            gl.getParameter(gl.MAX_VIEWPORT_DIMS),
            gl.getParameter(gl.MAX_VERTEX_ATTRIBS),
        ]
            .map((value) => String(value))
            .join("|");
    });
}

function fontFingerprint(): string {
    return safeValue(() => {
        const canvas = document.createElement("canvas");
        const context = canvas.getContext("2d");
        if (context === null) {
            return "";
        }

        const sample = "mmmmmmmmmmlli";
        context.font = "72px monospace";
        const baseline = context.measureText(sample).width;

        return FONT_CANDIDATES.map((font) => {
            context.font = `72px "${font}", monospace`;
            return context.measureText(sample).width === baseline ? "0" : "1";
        }).join("");
    });
}

export function collectClientFingerprint(): string {
    const navigatorWithHints = navigator as NavigatorWithHints;
    const screenInfo = window.screen;
    const hints = navigatorWithHints.userAgentData;
    const dateTimeOptions = Intl.DateTimeFormat().resolvedOptions();

    return [
        IDENTITY_VERSION,
        safeValue(() => navigatorWithHints.userAgent),
        safeValue(() => hints?.brands?.map((brand) => `${brand.brand}:${brand.version}`).join(",")),
        safeValue(() => hints?.mobile),
        safeValue(() => hints?.platform),
        safeValue(() => navigatorWithHints.platform),
        safeValue(() => navigatorWithHints.vendor),
        safeValue(() => navigatorWithHints.productSub),
        safeValue(() => navigatorWithHints.oscpu),
        safeValue(() => navigatorWithHints.language),
        safeValue(() => navigatorWithHints.languages),
        safeValue(() => navigatorWithHints.hardwareConcurrency),
        safeValue(() => navigatorWithHints.deviceMemory),
        safeValue(() => navigatorWithHints.maxTouchPoints),
        safeValue(() => navigatorWithHints.pdfViewerEnabled),
        safeValue(() => Array.from(navigatorWithHints.plugins).map((plugin) => plugin.name)),
        safeValue(() => screenInfo.width),
        safeValue(() => screenInfo.height),
        safeValue(() => screenInfo.availWidth),
        safeValue(() => screenInfo.availHeight),
        safeValue(() => screenInfo.colorDepth),
        safeValue(() => screenInfo.pixelDepth),
        safeValue(() => window.devicePixelRatio),
        safeValue(() => dateTimeOptions.timeZone),
        safeValue(() => dateTimeOptions.locale),
        safeValue(() => Intl.NumberFormat().resolvedOptions().locale),
        safeValue(() => Intl.Collator().resolvedOptions().locale),
        webGLFingerprint(),
        canvasFingerprint(),
        fontFingerprint(),
    ].join("|");
}

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

export function clientIdFromFingerprint(fingerprint: string): string {
    return formatClientID(fingerprint);
}

export function createClientId(): string | null {
    if (
        typeof window === "undefined" ||
        typeof document === "undefined" ||
        typeof navigator === "undefined"
    ) {
        return null;
    }

    return clientIdFromFingerprint(collectClientFingerprint());
}
