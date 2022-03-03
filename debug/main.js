import { Crdt } from "./crdt_view.js";

document.forms[0].addEventListener("submit", (evt) => {
  evt.preventDefault();
  let formData = new FormData(evt.target);
  let file = formData.get("content");

  readFile(file).then(parseJSONL).then(init);
});

function readFile(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = reject;
    reader.onload = () => resolve(reader.result);
    reader.readAsText(file);
  });
}

function parseJSONL(text) {
  return text
    .split("\n")
    .map((x) => x.trim())
    .filter((x) => x != "")
    .map(JSON.parse);
}

var crdt;
function init(states) {
  if (crdt !== undefined) {
    crdt.destroy();
  }
  crdt = new Crdt(states);
  crdt.render();
}
