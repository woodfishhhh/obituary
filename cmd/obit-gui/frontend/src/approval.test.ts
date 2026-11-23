import assert from "node:assert/strict";
import test from "node:test";
import { approvalReceipt } from "./approval.ts";

test("an approval preserves completed, failed and still pending candidate paths", () => {
  const results = [{ref:"#1", remaining:["retry"], results:[{ref:"ok",action:"added"},{ref:"gone",error:"missing"},{ref:"retry",error:"unreadable"}]}];
  for (const error of [undefined, "settlement warning"]) {
    const receipt = approvalReceipt({results, error}, "no reply");
    assert.equal(receipt.partial, true);
    assert.equal(receipt.error, error);
    assert.deepEqual(receipt.rows, [{ref:"ok",error:undefined,pending:false},{ref:"gone",error:"missing",pending:false},{ref:"retry",error:"unreadable",pending:true}]);
  }
  assert.equal(approvalReceipt({results:[]}, "no reply").error, "no reply");
});
