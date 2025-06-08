
function myFunc(first, second) {
    const innerVariable = 2
    return (first() + second()) * innerVariable
}

myFunc(function () { return 1 + 1 }, () => "2") + myFunc(() => 1 + 1, function () { const first = 1; const second = 1; return first + second })