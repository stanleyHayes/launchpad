import { ProductEvidence } from "./product-evidence";

/**
 * The home hero uses the same product evidence that appears throughout the
 * marketing site so the product promise is supported by a readable interface.
 */
export function ProductPreview() {
  return <ProductEvidence kind="journey" priority caption={false} />;
}
