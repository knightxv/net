export function stringToUint8Array(str: string) {
  var arr = [];
  for (var i = 0, j = str.length; i < j; ++i) {
    arr.push(str.charCodeAt(i));
  }

  var tmpUint8Array = new Uint8Array(arr);
  return tmpUint8Array;
}
export async function sleep(ms: number) {
  return new Promise((resolve) => {
    setTimeout(() => resolve(null), ms);
  });
}
