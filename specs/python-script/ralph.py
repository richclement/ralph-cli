import argparse
import json
from pathlib import Path
import re
import shlex
import subprocess
import textwrap


DEFAULT_COMPLETION_RESPONSE = "DONE"
DEFAULT_MAX_ITERATIONS = 10
DEFAULT_OUTPUT_TRUNCATE_CHARS = 5000
DEFAULT_STREAM_AGENT_OUTPUT = True


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Ralph loop harness.")
    prompt_group = parser.add_mutually_exclusive_group(required=True)
    prompt_group.add_argument("--prompt", help="Prompt string to send to agent.")
    prompt_group.add_argument(
        "--prompt-file", help="Path to file containing prompt text."
    )
    parser.add_argument(
        "--maximum-iterations",
        "-m",
        type=int,
        help="Maximum number of iterations before stopping.",
    )
    parser.add_argument(
        "--completion-response",
        "-c",
        help="Completion response text inside <response>...</response>.",
    )
    parser.add_argument(
        "--settings",
        default=str(Path(".ralph") / "settings.json"),
        help="Path to settings.json.",
    )
    parser.add_argument(
        "--stream-agent-output",
        action=argparse.BooleanOptionalAction,
        default=None,
        help="Stream agent output to the console (default: enabled).",
    )
    return parser.parse_args()


def load_settings(path: Path) -> dict:
    if not path.exists():
        return {}
    try:
        with path.open("r", encoding="utf-8") as handle:
            data = json.load(handle)
    except json.JSONDecodeError as exc:
        raise SystemExit(f"Invalid JSON in settings file: {path}") from exc
    if not isinstance(data, dict):
        raise SystemExit(f"Settings file must contain a JSON object: {path}")
    return data


def merge_settings_files(base: dict, override: dict) -> dict:
    merged = dict(base)
    for key, value in override.items():
        if (
            key in merged
            and isinstance(merged[key], dict)
            and isinstance(value, dict)
        ):
            merged[key] = merge_settings_files(merged[key], value)
        else:
            merged[key] = value
    return merged


def infer_non_repl_args(command: str) -> list[str]:
    command_lower = command.lower()
    if command_lower == "claude":
        return ["-p"]
    if command_lower == "codex":
        return ["e"]
    if command_lower == "amp":
        return ["-x"]
    return []


def load_prompt(args: argparse.Namespace) -> str:
    if args.prompt is not None:
        return args.prompt
    prompt_path = Path(args.prompt_file)
    try:
        return prompt_path.read_text(encoding="utf-8")
    except FileNotFoundError as exc:
        raise SystemExit(f"Prompt file not found: {prompt_path}") from exc


def merge_settings(args: argparse.Namespace, settings: dict) -> dict:
    config = dict(settings)
    config.setdefault("completionResponse", DEFAULT_COMPLETION_RESPONSE)
    config.setdefault("maximumIterations", DEFAULT_MAX_ITERATIONS)
    config.setdefault("outputTruncateChars", DEFAULT_OUTPUT_TRUNCATE_CHARS)
    config.setdefault("streamAgentOutput", DEFAULT_STREAM_AGENT_OUTPUT)

    if args.completion_response is not None:
        config["completionResponse"] = args.completion_response
    if args.maximum_iterations is not None:
        config["maximumIterations"] = args.maximum_iterations
    if args.stream_agent_output is not None:
        config["streamAgentOutput"] = args.stream_agent_output

    config["prompt"] = load_prompt(args)

    agent = dict(config.get("agent", {}))
    command = agent.get("command")
    if command:
        agent["nonReplArgs"] = infer_non_repl_args(command)
    config["agent"] = agent
    return config


def validate_settings(config: dict) -> None:
    errors = []

    if not isinstance(config.get("prompt"), str) or not config["prompt"].strip():
        errors.append("Prompt must be a non-empty string.")

    max_iterations = config.get("maximumIterations")
    if not isinstance(max_iterations, int) or max_iterations <= 0:
        errors.append("maximumIterations must be a positive integer.")

    completion_response = config.get("completionResponse")
    if not isinstance(completion_response, str) or not completion_response.strip():
        errors.append("completionResponse must be a non-empty string.")

    output_truncate_chars = config.get("outputTruncateChars")
    if not isinstance(output_truncate_chars, int) or output_truncate_chars <= 0:
        errors.append("outputTruncateChars must be a positive integer.")

    stream_agent_output = config.get("streamAgentOutput")
    if not isinstance(stream_agent_output, bool):
        errors.append("streamAgentOutput must be a boolean.")

    agent = config.get("agent")
    if agent is None:
        errors.append("agent is required.")
    elif not isinstance(agent, dict):
        errors.append("agent must be an object.")
    else:
        command = agent.get("command")
        if not isinstance(command, str) or not command.strip():
            errors.append("agent.command must be a non-empty string.")
        flags = agent.get("flags")
        if flags is not None:
            if not isinstance(flags, list) or not all(
                isinstance(item, str) for item in flags
            ):
                errors.append("agent.flags must be a list of strings.")
        non_repl_args = agent.get("nonReplArgs")
        if non_repl_args is not None:
            if not isinstance(non_repl_args, list) or not all(
                isinstance(item, str) for item in non_repl_args
            ):
                errors.append("agent.nonReplArgs must be a list of strings.")

    guardrails = config.get("guardrails")
    if guardrails is not None:
        if not isinstance(guardrails, list):
            errors.append("guardrails must be a list.")
        else:
            for index, guardrail in enumerate(guardrails):
                if not isinstance(guardrail, dict):
                    errors.append(f"guardrails[{index}] must be an object.")
                    continue
                command = guardrail.get("command")
                if not isinstance(command, str) or not command.strip():
                    errors.append(
                        f"guardrails[{index}].command must be a non-empty string."
                    )
                fail_action = guardrail.get("failAction")
                if fail_action is None:
                    errors.append(f"guardrails[{index}].failAction is required.")
                elif fail_action not in {"APPEND", "PREPEND", "REPLACE"}:
                    errors.append(
                        f"guardrails[{index}].failAction must be APPEND, PREPEND, or REPLACE."
                    )

    scm = config.get("scm")
    if scm is not None:
        if not isinstance(scm, dict):
            errors.append("scm must be an object.")
        else:
            command = scm.get("command")
            if command is not None and not isinstance(command, str):
                errors.append("scm.command must be a string.")
            tasks = scm.get("tasks")
            if tasks is not None:
                if not isinstance(tasks, list) or not all(
                    isinstance(item, str) for item in tasks
                ):
                    errors.append("scm.tasks must be a list of strings.")

    if errors:
        message = "\n".join(errors)
        raise SystemExit(f"Invalid settings:\n{message}")


def build_agent_command(agent: dict) -> list[str]:
    command = agent["command"]
    args = [command]
    args.extend(agent.get("nonReplArgs", []))
    for flag in agent.get("flags", []):
        args.extend(shlex.split(flag))
    return args


def ensure_ralph_dir(path: Path) -> None:
    path.mkdir(parents=True, exist_ok=True)


def slugify_command(command: str, max_length: int = 60) -> str:
    slug = re.sub(r"[^a-zA-Z0-9]+", "_", command).strip("_")
    if len(slug) > max_length:
        slug = slug[:max_length]
    return slug or "command"


def write_guardrail_log(
    ralph_dir: Path, iteration: int, command: str, output: str
) -> Path:
    slug = slugify_command(command)
    path = ralph_dir / f"guardrail_{iteration:03d}_{slug}.log"
    path.write_text(output, encoding="utf-8")
    return path


def run_agent(
    agent: dict,
    prompt: str,
    ralph_dir: Path,
    iteration: int,
    stream_output: bool,
) -> str:
    args = build_agent_command(agent)
    use_prompt_file = args[1:2] == ["e"] and agent["command"].lower() == "codex"

    if use_prompt_file:
        prompt_path = ralph_dir / f"prompt_{iteration:03d}.txt"
        prompt_path.write_text(prompt, encoding="utf-8")
        args.append(str(prompt_path))
    else:
        prompt_path = None

    if not stream_output:
        if use_prompt_file:
            result = subprocess.run(
                args,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
                check=False,
            )
        else:
            result = subprocess.run(
                args,
                input=prompt,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
                check=False,
            )
        return result.stdout

    process = subprocess.Popen(
        args,
        stdin=None if use_prompt_file else subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1,
    )
    if not use_prompt_file and process.stdin:
        process.stdin.write(prompt)
        process.stdin.close()

    output_chunks = []
    assert process.stdout is not None
    for line in process.stdout:
        output_chunks.append(line)
        print(line, end="", flush=True)
    process.wait()
    return "".join(output_chunks)


def parse_completion_response(output: str) -> str | None:
    match = re.search(
        r"<response>(.*?)</response>", output, flags=re.IGNORECASE | re.DOTALL
    )
    if not match:
        return None
    return match.group(1).strip()


def run_guardrails(config: dict, ralph_dir: Path, iteration: int) -> list[dict]:
    guardrails = config.get("guardrails") or []
    failures = []

    for guardrail in guardrails:
        command = guardrail["command"]
        fail_action = guardrail["failAction"]
        print(f"Guardrail start: {command}")
        result = subprocess.run(
            command,
            shell=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            check=False,
        )
        output = result.stdout
        log_path = write_guardrail_log(ralph_dir, iteration, command, output)
        if result.returncode != 0:
            print(
                f"Guardrail end: {command} (exit {result.returncode}, action={fail_action})"
            )
        else:
            print(f"Guardrail end: {command} (exit {result.returncode})")
        if result.returncode != 0:
            failures.append(
                {
                    "command": command,
                    "failAction": fail_action,
                    "exitCode": result.returncode,
                    "output": output,
                    "logPath": log_path,
                }
            )
        else:
            print("Guardrail passed")
    return failures


def apply_fail_action(prompt: str, failure_message: str, action: str) -> str:
    if action == "APPEND":
        return f"{prompt}\n\n{failure_message}"
    if action == "PREPEND":
        return f"{failure_message}\n\n{prompt}"
    if action == "REPLACE":
        return failure_message
    return prompt


def format_failure_message(config: dict, failure: dict) -> str:
    truncate_chars = config["outputTruncateChars"]
    output = failure["output"]
    truncated = output[:truncate_chars]
    summary = textwrap.dedent(
        f"""\
        Guardrail "{failure['command']}" failed with exit code {failure['exitCode']}.
        Output file: {failure['logPath']}
        Output (truncated):
        {truncated}
        """
    ).strip()
    return summary


def run_scm_tasks(config: dict, agent: dict, ralph_dir: Path, iteration: int) -> None:
    scm = config.get("scm")
    if not scm:
        return
    command = scm.get("command")
    tasks = scm.get("tasks") or []
    if not command or not tasks:
        return

    commit_message = None
    if "commit" in tasks:
        commit_prompt = (
            "Write a concise, imperative commit message for the current changes. "
            "Reply with only the message."
        )
        response = run_agent(
            agent,
            commit_prompt,
            ralph_dir,
            iteration,
            config["streamAgentOutput"],
        )
        for line in response.splitlines():
            if line.strip():
                commit_message = line.strip()
                break
        if not commit_message:
            print("SCM: skipping commit (no commit message returned)")

    for task in tasks:
        if task == "commit":
            if not commit_message:
                continue
            cmd = [command, "commit", "-am", commit_message]
        else:
            cmd = [command, *shlex.split(task)]
        print(f"SCM: running {' '.join(cmd)}")
        subprocess.run(cmd, check=False)


def main() -> int:
    args = parse_args()
    settings_path = Path(args.settings)
    settings = load_settings(settings_path)
    local_settings = load_settings(Path(".ralph") / "settings.local.json")
    settings = merge_settings_files(settings, local_settings)
    config = merge_settings(args, settings)
    validate_settings(config)

    ralph_dir = Path(".ralph")
    ensure_ralph_dir(ralph_dir)
    agent = config["agent"]
    base_prompt = config["prompt"]
    prompt = base_prompt
    completion_response = config["completionResponse"]
    max_iterations = config["maximumIterations"]

    for iteration in range(1, max_iterations + 1):
        print(f"\n=== Ralph iteration {iteration}/{max_iterations} ===")
        agent_output = run_agent(
            agent,
            prompt,
            ralph_dir,
            iteration,
            config["streamAgentOutput"],
        )

        failures = run_guardrails(config, ralph_dir, iteration)
        if failures:
            prompt = base_prompt
            for failure in failures:
                failure_message = format_failure_message(config, failure)
                prompt = apply_fail_action(
                    prompt, failure_message, failure["failAction"]
                )
            continue

        prompt = base_prompt

        response = parse_completion_response(agent_output)
        if response is not None and response.lower() == completion_response.lower():
            print(f"Completion response matched: {response}")
            run_scm_tasks(config, agent, ralph_dir, iteration)
            return 0

    print("Maximum iterations reached without completion response.")
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
