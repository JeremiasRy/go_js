const outer = () => {
    function f(a = b, b = 2) { return [a, b].join(","); }
    try { console.log(f()); } catch (e) { console.log(e.name); }
};
outer();
