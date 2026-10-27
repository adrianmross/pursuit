import { autofillDocument } from "./autofill";
import { extractJob } from "./extract";

type Msg = { type?: string; answers?: Record<string, string> };

chrome.runtime.onMessage.addListener((msg: Msg, _sender, sendResponse) => {
  if (msg?.type === "extract") {
    sendResponse({ job: extractJob(document, location.href) });
    return;
  }
  if (msg?.type === "autofill") {
    sendResponse(autofillDocument(document, msg.answers || {}));
    return true;
  }
});
