#!/usr/bin/env python3
"""Regenerate Eq. (53) reference values using 60-digit decimal arithmetic."""

import csv
from decimal import Decimal, localcontext
from pathlib import Path


def main():
    destination = Path(__file__).resolve().parents[1] / "testdata/thermal_conductivity.csv"
    temperatures = ("186.8673", "198.639", "216.650", "223.252", "250.350", "270.650", "273.150", "288.150", "300", "320.676")
    with localcontext() as context, destination.open("w", newline="") as output:
        context.prec = 60
        writer = csv.writer(output, lineterminator="\n")
        writer.writerow(("temperature_K", "thermal_conductivity_W_m_K", "source_page"))
        for text in temperatures:
            temperature = Decimal(text)
            # U.S. Standard Atmosphere 1976, Part 1, Eq. (53), printed page 19.
            conductivity = Decimal("2.64638e-3") * temperature * temperature.sqrt() / (
                temperature + Decimal("245.4") * Decimal(10) ** (-Decimal(12) / temperature)
            )
            writer.writerow((text, f"{conductivity:.20e}", 19))


if __name__ == "__main__":
    main()
