import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { TransferPage } from "./TransferPage";

describe("TransferPage", () => {
  it("shows Osmosis enabled and future chains disabled", () => {
    render(<TransferPage />);

    expect(screen.getByRole("button", { name: /osmosis testnet/i })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: /cosmos hub/i })).toBeDisabled();
  });

  it("updates the transfer form inputs and validates the osmosis recipient", async () => {
    const user = userEvent.setup();
    render(<TransferPage />);

    const amountInput = screen.getByLabelText(/amount/i);
    const recipientInput = screen.getByLabelText(/recipient/i);

    await user.clear(amountInput);
    await user.type(amountInput, "0.75");
    await user.clear(recipientInput);
    await user.type(recipientInput, "bad-recipient");

    expect(amountInput).toHaveValue("0.75");
    expect(screen.getByText(/enter a valid osmo1 recipient/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /bridge to osmosis/i }),
    ).toBeDisabled();
  });
});
