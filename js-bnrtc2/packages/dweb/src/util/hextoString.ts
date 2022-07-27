// hex转json字符串,16进制ASCII
export const hextoString = function (arr: Uint8Array) {
  const hexArr = [...arr].map((bit) => {
    return bit.toString(16);
  });
  return hexArr.join("");
};
