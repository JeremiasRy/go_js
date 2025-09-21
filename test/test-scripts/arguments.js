function showArgs() {
    let result = '';
    for (const key in arguments) {
        result += arguments[key] + ',';
    }
    console.log(result);
}

showArgs(1, 'two', true);
