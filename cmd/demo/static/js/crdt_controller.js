import { diff } from "./diff.js";

export class CrdtController {
  constructor(parent_controller, id, content) {
    this.id = id || crypto.randomUUID();
    this.parent_controller = parent_controller;
    this.view = null;
    this.content = content || "";
  }

  render() {
    this.view = $("<div>")
      .addClass("crdt")
      .append(
        $("<textarea>")
          .val(this.content)
          .on("input", (evt) => this.textInput(evt))
      )
      .append(
        $("<button>")
          .append("Fork")
          .click((evt) => this.fork(evt))
      )
      .append($("<div>").addClass("sync"));
    return this.view;
  }

  renderSyncArea() {
    let syncView = this.parent_controller.renderSyncArea(this);
    $(".sync", this.view).html("").append(syncView);
  }

  textarea() {
    return $("textarea", this.view).first();
  }

  textInput(evt) {
    let textarea = $(evt.target);
    let content = textarea.val();

    if (content == this.content) {
      return;
    }
    let ops = diff(this.content, content);
    this.content = content;
    console.log(ops);

    let body = { id: this.id, ops: ops };
    fetch("/edit", {
      method: "POST",
      headers: {
        Accept: "text/plain",
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    })
      .then((response) => response.text())
      .then((text) => this.handleEditResponse(text))
      .catch((err) => console.log(err));
  }

  handleEditResponse(text) {
    if (this.content != text) {
      console.log(
        `ERROR: ${this.id}: got ${text} from server (local: ${this.content})`
      );
    }
  }

  // NOTE: sync is just local to the current controller because it's more intuitive for anyone
  // clicking a button in its scope, so it merges in the changes from incoming-connected sites.
  //
  // We may rethink this if we get a better UX to show what is changing in remote controllers.
  sync() {
    let mergeIds = this.parent_controller.incomingIds(this);

    let body = { id: this.id, mergeIds: mergeIds };
    fetch("/sync", {
      method: "POST",
      headers: {
        Accept: "text/plain",
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    })
      .then((response) => response.text())
      .then((text) => this.handleSyncResponse(text))
      .catch((err) => console.log(err));
  }

  handleSyncResponse(text) {
    this.textarea().val(text);
    this.content = text;
  }

  fork() {
    let body = { local: this.id };
    fetch("/fork", {
      method: "POST",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    })
      .then((response) => response.json())
      .then((json) => this.handleForkResponse(json))
      .catch((err) => console.log(err));
  }

  handleForkResponse(tree) {
    let sibling = this.parent_controller.newCrdt(tree.id, tree.content);

    this.parent_controller.connect(this.id, sibling.id);
    this.renderSyncArea();
    sibling.renderSyncArea();

    let textarea1 = this.textarea();
    let textarea2 = sibling.textarea();

    // Copy properties of this textarea to forked textarea.
    textarea2.val(textarea1.val());
    textarea2.prop("selectionStart", textarea1.prop("selectionStart"));
    textarea2.prop("selectionEnd", textarea1.prop("selectionEnd"));
    textarea2.prop("selectionDirection", textarea1.prop("selectionDirection"));
  }
}
