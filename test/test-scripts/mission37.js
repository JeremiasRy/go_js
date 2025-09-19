function showArgs() {
    let result = '';
    for (const key in arguments) {
        result += arguments[key] + ',';
    }
    console.log(result.slice(0, -1));
}

showArgs(1, 'two', true);