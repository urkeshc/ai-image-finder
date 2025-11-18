import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns
import os
import numpy as np
import pathlib # Added pathlib

# Determine paths relative to this script's location
script_dir = pathlib.Path(__file__).resolve().parent # Should be proj3/scripts
project_root_from_script = script_dir.parent       # Should be proj3

# Default paths relative to the project root (proj3/)
default_csv_filepath = project_root_from_script / "results" / "results.csv"
default_output_dir = project_root_from_script / "plots"

def plot_speedup(csv_filepath=default_csv_filepath, output_dir=default_output_dir): # Updated defaults
    """
    Reads benchmark results from a CSV file, calculates speedup,
    and generates speedup plots for each dataset size and mode.
    """
    if not os.path.exists(csv_filepath):
        print(f"Error: CSV file not found at {csv_filepath}")
        return

    try:
        df = pd.read_csv(csv_filepath)
    except pd.errors.EmptyDataError:
        print(f"Error: CSV file {csv_filepath} is empty.")
        return
    except Exception as e:
        print(f"Error reading CSV file {csv_filepath}: {e}")
        return

    if df.empty:
        print(f"CSV file {csv_filepath} is empty or contains no data after header.")
        return

    print("CSV Data Head:")
    print(df.head())

    # Ensure correct data types
    df['threads'] = df['threads'].astype(int)
    df['size'] = df['size'].astype(int)
    df['time_ms'] = df['time_ms'].astype(float)

    # Create output directory if it doesn't exist
    os.makedirs(output_dir, exist_ok=True)
    print(f"Plots will be saved to: {os.path.abspath(output_dir)}")

    dataset_sizes = df['size'].unique()
    modes = df['mode'].unique()

    for size_val in dataset_sizes:
        plt.figure(figsize=(12, 8))
        size_df = df[df['size'] == size_val].copy()

        if size_df.empty:
            print(f"No data found for dataset size: {size_val}")
            continue

        # Find the sequential time (mode='seq', threads=1) for this size as baseline
        seq_run = size_df[(size_df['mode'] == 'seq') & (size_df['threads'] == 1)]

        if seq_run.empty:
            print(f"Warning: Sequential baseline (mode='seq', threads=1) not found for size {size_val}.")
            print("Speedup will be calculated relative to the 1-thread performance of each parallel mode.")
            
            for mode_val in modes:
                if mode_val == 'seq':
                    continue # Skip seq mode itself if its baseline is missing for others

                mode_specific_df = size_df[size_df['mode'] == mode_val].copy()
                if mode_specific_df.empty:
                    continue

                baseline_t1_run = mode_specific_df[mode_specific_df['threads'] == 1]
                if not baseline_t1_run.empty:
                    baseline_t1_time = baseline_t1_run['time_ms'].iloc[0]
                    if baseline_t1_time > 0:
                        mode_specific_df['speedup'] = baseline_t1_time / mode_specific_df['time_ms']
                        sns.lineplot(data=mode_specific_df, x='threads', y='speedup', label=f'{mode_val} (vs {mode_val} T1)', marker='o', errorbar=None)
                    else:
                        print(f"  Baseline T1 time for {mode_val} at size {size_val} is 0 or less, cannot calculate speedup.")
                else:
                    print(f"  1-thread run for mode '{mode_val}' at size {size_val} not found. Cannot calculate speedup for this mode.")
        else:
            sequential_time = seq_run['time_ms'].iloc[0]
            print(f"Sequential baseline for size {size_val}: {sequential_time:.2f} ms")

            if sequential_time <= 0:
                print(f"Warning: Sequential baseline time for size {size_val} is {sequential_time} ms. Cannot calculate meaningful speedup.")
                # Plot raw times instead or skip
                plt.close()
                continue


            # Calculate speedup for all modes relative to sequential
            # For 'seq' mode itself, speedup will be 1 (or slightly off due to float precision if not handled)
            size_df['speedup'] = sequential_time / size_df['time_ms']
            
            # Plot for each mode
            for mode_val in modes:
                mode_data = size_df[size_df['mode'] == mode_val]
                if mode_data.empty:
                    continue
                if mode_val == 'seq':
                     # Ensure 'seq' is plotted as a flat line at 1 if it's the baseline
                    seq_threads = mode_data['threads'].unique()
                    sns.lineplot(x=seq_threads, y=[1.0]*len(seq_threads), label=f'{mode_val} (Baseline)', linestyle='--', color='gray', marker='o')
                else:
                    sns.lineplot(data=mode_data, x='threads', y='speedup', label=mode_val, marker='o', errorbar=None)
        
        # Ideal speedup line
        # Find max threads plotted for this size, excluding seq if it only has 1 thread point
        # and other modes have more.
        plotted_threads = size_df[size_df['mode'] != 'seq']['threads'].unique()
        if len(plotted_threads) == 0 : # if only seq mode was run or no parallel modes
            plotted_threads = size_df['threads'].unique()

        if len(plotted_threads) > 0:
            max_threads_for_ideal_line = np.max(plotted_threads)
            if max_threads_for_ideal_line > 0:
                 plt.plot([1, max_threads_for_ideal_line], [1, max_threads_for_ideal_line], 
                          label='Ideal Speedup', linestyle=':', color='black', alpha=0.7)

        plt.title(f'Speedup vs. Threads (Dataset Size: {size_val:,})')
        plt.xlabel('Number of Threads/Workers')
        plt.ylabel(f'Speedup (Baseline: Sequential Time or T1 of mode)')
        
        # Set x-axis ticks to be the actual thread counts tested
        all_tested_threads = sorted(df['threads'].unique())
        plt.xticks(all_tested_threads)
        
        plt.grid(True, which="both", ls="-", alpha=0.5)
        plt.legend(title="Mode")
        plt.tight_layout()
        
        plot_filename = os.path.join(output_dir, f'speedup_size_{size_val}.png')
        plt.savefig(plot_filename)
        print(f"Plot saved to {plot_filename}")
        plt.close()

if __name__ == '__main__':
    # Ensure matplotlib, seaborn, and pandas are installed:
    # pip install pandas matplotlib seaborn numpy pathlib
    plot_speedup()
    print("Plotting complete.")

