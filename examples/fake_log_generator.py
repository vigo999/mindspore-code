#!/usr/bin/env python3
import argparse
import json
import random
import time


def clamp(value: float, low: float, high: float) -> float:
    return max(low, min(high, value))


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Emit fake training logs for /train dashboard verification.")
    parser.add_argument("--run-id", default="fake-run")
    parser.add_argument("--host", default="gpu")
    parser.add_argument("--model", default="qwen2.5-7b-fake")
    parser.add_argument("--total-steps", type=int, default=120)
    parser.add_argument("--interval-seconds", type=float, default=0.2)
    parser.add_argument("--seed", type=int, default=None)
    return parser.parse_args()


def main() -> None:
    args = parse_args()

    total_steps = max(args.total_steps, 1)
    interval_seconds = max(args.interval_seconds, 0.01)

    seed = args.seed
    if seed is None:
        seed = sum(ord(ch) for ch in f"{args.run_id}:{args.host}:{args.model}")
    rng = random.Random(seed)

    start_loss = 3.8 + rng.uniform(-0.15, 0.15)
    end_loss = 0.35 + rng.uniform(-0.04, 0.04)

    start_grad = 12.0 + rng.uniform(-0.5, 0.5)
    end_grad = 0.8 + rng.uniform(-0.1, 0.1)

    start_throughput = 420.0 + rng.uniform(-40.0, 40.0)
    end_throughput = 780.0 + rng.uniform(-50.0, 50.0)

    start_lr = 2.0e-5
    end_lr = 5.0e-6

    try:
        for step in range(1, total_steps + 1):
            progress = clamp(step / total_steps, 0.0, 1.0)

            loss_trend = start_loss + (end_loss - start_loss) * progress
            loss_noise = rng.uniform(-0.08, 0.08) * (1.0 - 0.7 * progress)
            loss = max(end_loss * 0.7, loss_trend + loss_noise)

            grad_trend = start_grad + (end_grad - start_grad) * progress
            grad_noise = rng.uniform(-0.3, 0.3) * (1.0 - 0.6 * progress)
            grad_norm = max(0.1, grad_trend + grad_noise)

            throughput_trend = start_throughput + (end_throughput - start_throughput) * progress
            throughput_noise = rng.uniform(-12.0, 12.0)
            throughput = max(50.0, throughput_trend + throughput_noise)

            learning_rate = start_lr + (end_lr - start_lr) * progress
            epoch = step * 0.01

            payload = {
                "run_id": args.run_id,
                "host": args.host,
                "model": args.model,
                "step": step,
                "total_steps": total_steps,
                "loss": round(loss, 5),
                "throughput": round(throughput, 2),
                "grad_norm": round(grad_norm, 4),
                "learning_rate": f"{learning_rate:.5e}",
                "epoch": round(epoch, 2),
            }
            print(json.dumps(payload, sort_keys=True), flush=True)

            time.sleep(interval_seconds)
        print(
            json.dumps(
                {
                    "run_id": args.run_id,
                    "host": args.host,
                    "model": args.model,
                    "status": "completed",
                    "step": total_steps,
                    "total_steps": total_steps,
                },
                sort_keys=True,
            ),
            flush=True,
        )
    except (KeyboardInterrupt, BrokenPipeError):
        pass


if __name__ == "__main__":
    main()
