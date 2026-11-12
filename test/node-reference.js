// Execute one reference case for the Go/Docker test runner.
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { pathToFileURL, fileURLToPath } = require("node:url");

const resultMarker = "\n@@GO_JS_TEST_RESULT@@";

function moduleLoader() {
    const records = new Map();
    const parseErrors = new WeakSet();

    function load(filename) {
        filename = path.resolve(filename);
        if (records.has(filename)) return records.get(filename);
        const source = fs.readFileSync(filename, "utf8");
        const record = { module: null, linking: null, evaluation: null };
        try {
            record.module = new vm.SourceTextModule(source, {
                identifier: pathToFileURL(filename).href,
                initializeImportMeta(meta, module) { meta.url = module.identifier; },
                async importModuleDynamically(specifier, referringModule) {
                    const dependency = load(resolve(specifier, referringModule));
                    await link(dependency);
                    await evaluate(dependency);
                    return dependency.module;
                }
            });
        } catch (error) {
            parseErrors.add(error);
            throw error;
        }
        records.set(filename, record);
        return record;
    }

    function resolve(specifier, referringModule) {
        return fileURLToPath(new URL(specifier, referringModule.identifier));
    }

    async function link(record) {
        if (!record.linking && record.module.status === "unlinked") {
            record.linking = record.module.link((specifier, referringModule) => load(resolve(specifier, referringModule)).module);
        }
        if (record.linking) await record.linking;
        if (record.module.status === "errored") throw record.module.error;
    }

    async function evaluate(record) {
        if (!record.evaluation) record.evaluation = record.module.evaluate();
        return record.evaluation;
    }

    return { load, link, evaluate, parseErrors };
}

async function runCase(filename, sourceGoal) {
    let phase = "parse";
    const loader = moduleLoader();
    try {
        if (sourceGoal === "module") {
            const record = loader.load(filename);
            phase = "link";
            await loader.link(record);
            phase = "runtime";
            await loader.evaluate(record);
        } else {
            const script = new vm.Script(fs.readFileSync(filename, "utf8"), { filename });
            phase = "runtime";
            // A genuine Script in a fresh child process: no CommonJS wrapper or
            // contextified sandbox changing global-var property descriptors.
            await script.runInThisContext();
        }
        return { outcome: "success" };
    } catch (error) {
        return {
            outcome: "error",
            phase: loader.parseErrors.has(error) ? "parse" : phase,
            error: error && error.name,
            message: error && error.message
        };
    }
}

async function main() {
    const [sourceGoal, filename, ...extra] = process.argv.slice(2);
    if (!filename || extra.length || !["script", "module"].includes(sourceGoal)) {
        throw new Error("Expected a source goal and JavaScript file from the Go test runner");
    }
    const result = await runCase(path.resolve(filename), sourceGoal);
    // Legacy tests schedule timers without returning their work. Emit the
    // protocol trailer after their callbacks and nested jobs have drained.
    process.once("beforeExit", () => {
        process.stdout.write(resultMarker + JSON.stringify(result) + "\n");
    });
}

main().catch(error => { console.error(error.message); process.exitCode = 1; });
