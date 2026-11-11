// M3: TimeClip boundaries, invalid dates, ISO errors, and JSON conversion.
for (const input of [NaN, Infinity, "invalid date", 8640000000000001, -8640000000000001]) {
    const date = new Date(input);
    console.log("invalid", Number.isNaN(date.getTime()), Number.isNaN(date.getUTCFullYear()), date.toString(), date.toJSON());
    try { date.toISOString(); } catch (error) { console.log("iso-error", error.name); }
}
console.log("boundaries", new Date(8640000000000000).getTime(), new Date(-8640000000000000).getTime());
console.log("clip-fraction", new Date(1.9).getTime(), new Date(-1.9).getTime());
console.log("invalid-parse", Number.isNaN(Date.parse("not-a-date")), Number.isNaN(Date.UTC(NaN, 0)));
const repaired = new Date(NaN);
repaired.setTime(0);
console.log("repair", repaired.toISOString());
