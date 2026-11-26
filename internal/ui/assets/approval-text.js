(function (root, factory) {
  const displayApprovalText = factory();
  if (typeof module === 'object' && module.exports) {
    module.exports = displayApprovalText;
  } else {
    root.displayApprovalText = displayApprovalText;
  }
})(globalThis, function () {
  return function displayApprovalText(value) {
    const raw = String(value ?? '');
    let escaped = false;
    const text = raw.replace(/[\p{Cc}\p{Cf}]/gu, character => {
      if (character === '\n' || character === '\t') return character;
      escaped = true;
      return `\\u{${character.codePointAt(0).toString(16).toUpperCase().padStart(4, '0')}}`;
    });
    return { text, escaped };
  };
});
