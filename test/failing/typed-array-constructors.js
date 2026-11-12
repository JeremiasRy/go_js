for (const name of ["Float32Array", "Int32Array", "Uint32Array", "Uint16Array", "Uint8ClampedArray"]) {
    try {
        const values = name === "Float32Array" ? [1.5] : [1];
        const C = { Float32Array, Int32Array, Uint32Array, Uint16Array, Uint8ClampedArray }[name];
        console.log(name, new C(values).length);
    } catch (e) { console.log(name, e.name); }
}
