class JunglePitchShifter extends AudioWorkletProcessor {
    constructor() {
        super();
        this.pitchMultiplier = 1.6; // Female voice simulation
        this.bufferTime = 0.100; // 100ms
        this.fadeTime = 0.050; // 50ms
        this.bufferLength = Math.floor(sampleRate * this.bufferTime);
        this.fadeLength = Math.floor(sampleRate * this.fadeTime);
        this.buffer = new Float32Array(this.bufferLength);
        this.writePointer = 0;
        this.read1 = 0;
        this.read2 = this.bufferLength / 2;
    }

    process(inputs, outputs, parameters) {
        const input = inputs[0];
        const output = outputs[0];

        if (!input || !input[0] || !output || !output[0]) return true;

        const inputChannel = input[0];
        const outputChannel = output[0];

        for (let i = 0; i < inputChannel.length; i++) {
            // Write to buffer
            this.buffer[this.writePointer] = inputChannel[i];

            // Read from buffer with pitch multiplier
            this.read1 += this.pitchMultiplier;
            if (this.read1 >= this.bufferLength) this.read1 -= this.bufferLength;
            this.read2 += this.pitchMultiplier;
            if (this.read2 >= this.bufferLength) this.read2 -= this.bufferLength;

            let val1 = this.buffer[Math.floor(this.read1)];
            let val2 = this.buffer[Math.floor(this.read2)];

            // Fading logic to avoid clicks
            let pos1 = this.read1 / this.bufferLength;
            let fade1 = pos1 < 0.5 ? pos1 * 2 : (1 - pos1) * 2;
            let pos2 = this.read2 / this.bufferLength;
            let fade2 = pos2 < 0.5 ? pos2 * 2 : (1 - pos2) * 2;

            outputChannel[i] = (val1 * fade1 + val2 * fade2) / (fade1 + fade2 || 1);

            this.writePointer++;
            if (this.writePointer >= this.bufferLength) {
                this.writePointer = 0;
            }
        }

        // Copy to other channels if stereo
        for (let c = 1; c < output.length; c++) {
            output[c].set(outputChannel);
        }

        return true;
    }
}
registerProcessor('jungle-pitch-shifter', JunglePitchShifter);
